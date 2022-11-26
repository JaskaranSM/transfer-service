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

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/jaskaranSM/transfer-service/logging"
	gdriveconstants "github.com/jaskaranSM/transfer-service/service/gdrive/constants"

	"github.com/jaskaranSM/transfer-service/constants"
	"github.com/jaskaranSM/transfer-service/utils"
)

func NewGoogleDriveFileTransfer(service *drive.Service, listener FileTransferListener, cb func(*drive.File)) *GoogleDriveFileTransfer {
	return &GoogleDriveFileTransfer{
		service:            service,
		listener:           listener,
		onTransferComplete: cb,
	}
}

type GoogleDriveFileTransfer struct {
	service            *drive.Service
	completed          int64
	file               *os.File
	fileId             string
	transferType       string
	isClone            bool
	listener           FileTransferListener
	isCancelled        bool
	onTransferComplete func(*drive.File)
}

func (g *GoogleDriveFileTransfer) clean() {
	logger := logging.GetLogger()
	g.completed = 0
	_, err := g.file.Seek(0, 0)
	if err != nil {
		logger.Error("Error while seeking file handle", zap.Error(err))
	}
}

func (g *GoogleDriveFileTransfer) CompletedLength() int64 {
	return g.completed
}

func (g *GoogleDriveFileTransfer) Write(p []byte) (int, error) {
	logger := logging.GetLogger()
	if g.isCancelled {
		err := constants.CancelledByUserError
		g.listener.OnTransferError(g, err)
		return 0, err
	}
	bytesWritten, err := g.file.Write(p)
	g.completed += int64(bytesWritten)
	logger.Debug("on transfer update: ", zap.Int("chunk_written", bytesWritten))
	g.listener.OnTransferUpdate(g, int64(bytesWritten))
	if err != nil && err != io.EOF {
		logger.Error("Error while writing file bytes", zap.Error(err), zap.String("filepath", g.file.Name()))
		g.listener.OnTransferError(g, err)
	}
	return bytesWritten, err
}

func (g *GoogleDriveFileTransfer) Read(p []byte) (int, error) {
	logger := logging.GetLogger()
	if g.isCancelled {
		err := errors.New("Cancelled by user.")
		g.listener.OnTransferError(g, err)
		return 0, err
	}
	bytesRead, err := g.file.Read(p)
	g.completed += int64(bytesRead)
	logger.Debug("on transfer update: ", zap.Int("chunk_read", bytesRead))
	g.listener.OnTransferUpdate(g, int64(bytesRead))
	if err != nil && err != io.EOF {
		logger.Error("Error while reading file bytes", zap.Error(err), zap.String("filepath", g.file.Name()))
		g.listener.OnTransferError(g, err)
	}
	return bytesRead, err
}

func (g *GoogleDriveFileTransfer) Cancel() {
	g.isCancelled = true
}

func (g *GoogleDriveFileTransfer) Clone(file *drive.File, desId string, retry int) {
	logger := logging.GetLogger()
	g.transferType = gdriveconstants.TransferTypeCloning
	logger.Info("on transfer start", zap.String("fileID", file.Id))
	g.listener.OnTransferStart(g)
	f := &drive.File{
		Parents: []string{desId},
	}
	file, err := g.service.Files.Copy(file.Id, f).SupportsAllDrives(true).SupportsTeamDrives(true).Do()
	if err != nil {
		logger.Error("Error while copying file", zap.Error(err), zap.String("fileID", file.Id))
		g.listener.OnTransferError(g, err)
		return
	}
	g.listener.OnTransferUpdate(g, file.Size)
	g.fileId = file.Id
	g.onTransferComplete(file)
	logger.Info("on transfer complete", zap.String("fileID", file.Id))
	g.listener.OnTransferComplete(g)
}

func (g *GoogleDriveFileTransfer) Download(file *drive.File, path string, retry int) {
	logger := logging.GetLogger()
	g.transferType = gdriveconstants.TransferTypeDownloading
	fileHandle, err := os.Create(path)
	if err != nil {
		logger.Error("Error while opening file handle",
			zap.Error(err),
			zap.String("filepath", path),
		)
		g.listener.OnTransferError(g, err)
		return
	}
	g.file = fileHandle
	if retry == 0 {
		logger.Debug("on Transfer Start:", zap.String("fileID", file.Id))
		g.listener.OnTransferStart(g)
	}
	res, err := g.service.Files.Get(file.Id).SupportsAllDrives(true).SupportsTeamDrives(true).Download()
	if err != nil {
		logger.Error("Error while getting file", zap.Error(err))
		g.listener.OnTransferError(g, err)
		return
	}
	defer res.Body.Close()
	_, err = io.Copy(g, res.Body)
	if err != nil {
		logger.Error("Error while copying file stream", zap.Error(err))
		g.listener.OnTransferError(g, err)
		return
	}
	logger.Debug("on transfer complete", zap.String("path", path))
	g.listener.OnTransferComplete(g)
}

func (g *GoogleDriveFileTransfer) Upload(path string, parentId string, retry int) {
	logger := logging.GetLogger()
	g.transferType = gdriveconstants.TransferTypeUploading
	fileHandle, err := os.Open(path)
	if err != nil {
		logger.Error("Error while opening file handle",
			zap.Error(err),
			zap.String("filepath", path),
		)
		g.listener.OnTransferError(g, err)
		return
	}
	g.file = fileHandle
	if retry == 0 {
		logger.Debug("on Transfer Start:", zap.String("path", path))
		g.listener.OnTransferStart(g)
	}
	contentType, err := utils.GetFileContentTypePath(path)
	if err != nil {
		logger.Error("Error while getting mimetype of file", zap.Error(err))
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
		logger.Error("Error creating file on gdrive", zap.Error(err))
		g.listener.OnTransferError(g, err)
		return
	}
	g.fileId = file.Id
	g.onTransferComplete(file)
	g.listener.OnTransferComplete(g)
}

func NewGoogleDriveClient(con int, total int64, listener GoogleDriveClientListener) *GoogleDriveClient {
	client := &GoogleDriveClient{
		CredentialFile: "credentials.json",
		TokenFile:      "token.json",
		concurrency:    make(chan int, con),
		total:          total,
		listener:       listener,
		Name:           "unknown",
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
	logger := logging.GetLogger()
	var client *http.Client
	if sa {
		b, err := ioutil.ReadFile(fmt.Sprintf("%s/%d.json", gdriveconstants.SADir, gdriveconstants.GlobalSAIndex))
		if err != nil {
			logger.Error("Error reading service account file", zap.Error(err))
			return nil, err
		}
		config, err := google.JWTConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			logger.Error("Error parsing JWT config from json", zap.Error(err))
			return nil, err
		}
		client = config.Client(context.Background())
	} else {
		b, err := ioutil.ReadFile(gd.CredentialFile)
		if err != nil {
			logger.Error("Error reading credentials file", zap.Error(err))
			return nil, err
		}
		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.ConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			logger.Error("Error parsing google account config from json", zap.Error(err))
			return nil, err
		}
		client = gd.getClient(config)
	}
	return client, nil
}
