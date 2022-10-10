package gdrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	gdriveconstants "github.com/jaskaranSM/transfer-service/service/gdrive/constants"

	"github.com/jaskaranSM/transfer-service/constants"
	"github.com/jaskaranSM/transfer-service/utils"
)

func NewGoogleDriveFileTransfer(service *drive.Service, listener FileTransferListener, cb func(*drive.File)) *GoogleDriveFileTransfer {
	return &GoogleDriveFileTransfer{
		service:          service,
		listener:         listener,
		onUploadComplete: cb,
	}
}

type GoogleDriveFileTransfer struct {
	service          *drive.Service
	completed        int64
	file             *os.File
	fileId           string
	isUpload         bool
	isClone          bool
	listener         FileTransferListener
	isCancelled      bool
	onUploadComplete func(*drive.File)
}

func (g *GoogleDriveFileTransfer) clean() {
	g.completed = 0
	g.file.Seek(0, 0)
}

func (g *GoogleDriveFileTransfer) CompletedLength() int64 {
	return g.completed
}

func (g *GoogleDriveFileTransfer) Write(p []byte) (int, error) {
	if g.isCancelled {
		err := constants.CancelledByUserError
		g.listener.OnTransferError(g, err)
		return 0, err
	}
	bytesWritten, err := g.file.Write(p)
	g.completed += int64(bytesWritten)
	g.listener.OnTransferUpdate(g, int64(bytesWritten))
	if err != nil && err != io.EOF {
		g.listener.OnTransferError(g, err)
	}
	return bytesWritten, err
}

func (g *GoogleDriveFileTransfer) Read(p []byte) (int, error) {
	if g.isCancelled {
		err := errors.New("Cancelled by user.")
		g.listener.OnTransferError(g, err)
		return 0, err
	}
	bytesRead, err := g.file.Read(p)
	g.completed += int64(bytesRead)
	g.listener.OnTransferUpdate(g, int64(bytesRead))
	if err != nil && err != io.EOF {
		g.listener.OnTransferError(g, err)
	}
	return bytesRead, err
}

func (g *GoogleDriveFileTransfer) Cancel() {
	g.isCancelled = true
}

func (g *GoogleDriveFileTransfer) Clone(file *drive.File, desId string, retry int) {
	g.isClone = true
	if retry == 0 {
		g.listener.OnTransferStart(g)
	}
	f := &drive.File{
		Parents: []string{desId},
	}
	file, err := g.service.Files.Copy(file.Id, f).SupportsAllDrives(true).SupportsTeamDrives(true).Do()
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	g.listener.OnTransferComplete(g)
}

func (g *GoogleDriveFileTransfer) Download(file *drive.File, path string, retry int) {
	g.isUpload = false
	fileHandle, err := os.Create(path)
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	g.file = fileHandle
	if retry == 0 {
		g.listener.OnTransferStart(g)
	}
	res, err := g.service.Files.Get(file.Id).SupportsAllDrives(true).SupportsTeamDrives(true).Download()
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	defer res.Body.Close()
	_, err = io.Copy(g, res.Body)
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	g.listener.OnTransferComplete(g)
}

func (g *GoogleDriveFileTransfer) Upload(path string, parentId string, retry int) {
	g.isUpload = true
	fileHandle, err := os.Open(path)
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	g.file = fileHandle
	if retry == 0 {
		g.listener.OnTransferStart(g)
	}
	contentType, err := utils.GetFileContentTypePath(path)
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	f := &drive.File{
		MimeType: contentType,
		Name:     filepath.Base(path),
		Parents:  []string{parentId},
	}
	file, err := g.service.Files.Create(f).SupportsAllDrives(true).SupportsTeamDrives(true).Media(g, googleapi.ChunkSize(512*1024)).Do()
	if err != nil {
		g.listener.OnTransferError(g, err)
		return
	}
	g.fileId = file.Id
	g.onUploadComplete(file)
	g.listener.OnTransferComplete(g)
}

func NewGoogleDriveClient(con int, total int64, listener GoogleDriveClientListener) *GoogleDriveClient {
	client := &GoogleDriveClient{
		CredentialFile: "credentials.json",
		TokenFile:      "token.json",
		concurrency:    make(chan int, con),
		total:          total,
		listener:       listener,
	}
	return client
}

func (gd *GoogleDriveClient) Authorize() error {
	srv, err := gd.GetDriveService()
	if err != nil {
		return err
	}
	gd.DriveSrv = srv
	return nil
}

func (gd *GoogleDriveClient) CompletedLength() int64 {
	return gd.completed
}

func (gd *GoogleDriveClient) TotalLength() int64 {
	return gd.total
}

func (gd *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	tok, err := gd.tokenFromFile(gd.TokenFile)
	if err != nil {
		tok = gd.getTokenFromWeb(config)
		gd.saveToken(gd.TokenFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func (gd *GoogleDriveClient) getAuthorizedHTTPClient(sa bool) (*http.Client, error) {
	var client *http.Client
	if sa {
		b, err := ioutil.ReadFile(fmt.Sprintf("%s/%d.json", gdriveconstants.SADir, gdriveconstants.GlobalSAIndex))
		config, err := google.JWTConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			return nil, err
		}
		client = config.Client(context.Background())
	} else {
		b, err := ioutil.ReadFile(gd.CredentialFile)
		if err != nil {
			return nil, err
		}
		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.ConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			return nil, err
		}
		client = gd.getClient(config)
	}
	return client, nil
}
