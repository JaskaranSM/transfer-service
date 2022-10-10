package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

var GLOBAL_SA_INDEX int = 0
var SA_DIR string = "accounts"

type FileTransferListener interface {
	OnTransferStart(*GoogleDriveFileTransfer)
	OnTransferUpdate(*GoogleDriveFileTransfer, int64)
	OnTransferComplete(*GoogleDriveFileTransfer)
	OnTransferTemporaryError(*GoogleDriveFileTransfer, error)
	OnTransferError(*GoogleDriveFileTransfer, error)
}

type GoogleDriveClientListener interface {
	OnTransferStart(*GoogleDriveClient)
	OnTransferComplete(*GoogleDriveClient, string)
	OnTransferError(*GoogleDriveClient, error)
}

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
		err := errors.New("Cancelled by user.")
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
	contentType, err := GetFileContentTypePath(path)
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

type GoogleDriveClient struct {
	CredentialFile       string
	TokenFile            string
	concurrency          chan int
	currentTransferQueue []*GoogleDriveFileTransfer
	listener             GoogleDriveClientListener
	mut                  sync.Mutex
	completed            int64
	total                int64
	isCancelled          bool
	callbackFired        bool
	fileId               string
	wg                   sync.WaitGroup
	DriveSrv             *drive.Service
}

func (G *GoogleDriveClient) Authorize() error {
	srv, err := G.GetDriveService()
	if err != nil {
		return err
	}
	G.DriveSrv = srv
	return nil
}

func (G *GoogleDriveClient) CompletedLength() int64 {
	return G.completed
}

func (G *GoogleDriveClient) TotalLength() int64 {
	return G.total
}

func (G *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	tok, err := G.tokenFromFile(G.TokenFile)
	if err != nil {
		tok = G.getTokenFromWeb(config)
		G.saveToken(G.TokenFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func (G *GoogleDriveClient) getAuthorizedHTTPClient(sa bool) (*http.Client, error) {
	var client *http.Client
	if sa {
		b, err := ioutil.ReadFile(fmt.Sprintf("%s/%d.json", SA_DIR, GLOBAL_SA_INDEX))
		config, err := google.JWTConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			return nil, err
		}
		client = config.Client(context.Background())
	} else {
		b, err := ioutil.ReadFile(G.CredentialFile)
		if err != nil {
			return nil, err
		}
		// If modifying these scopes, delete your previously saved token.json.
		config, err := google.ConfigFromJSON(b, drive.DriveScope)
		if err != nil {
			return nil, err
		}
		client = G.getClient(config)
	}
	return client, nil
}

func (G *GoogleDriveClient) GetDriveService() (*drive.Service, error) {
	client, err := G.getAuthorizedHTTPClient(true)
	if err != nil {
		return nil, err
	}
	srv, err := drive.New(client)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func (G *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func (G *GoogleDriveClient) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func (G *GoogleDriveClient) saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (G *GoogleDriveClient) CreateDir(name string, parentId string) (*drive.File, error) {
	d := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentId},
	}
	file, err := G.DriveSrv.Files.Create(d).SupportsAllDrives(true).Do()
	return file, err
}

func (G *GoogleDriveClient) OnTransferError(transfer *GoogleDriveFileTransfer, err error) {
	fmt.Printf("[OnTransferError]: %v, %v\n", transfer, err)
	<-G.concurrency
	G.wg.Done()
	if G.callbackFired {
		return
	}
	G.callbackFired = true
	G.listener.OnTransferError(G, err)
}

func (G *GoogleDriveClient) OnTransferStart(transfer *GoogleDriveFileTransfer) {
	fmt.Printf("[OnTransferStart]: %s\n", transfer.file.Name())
}

func (G *GoogleDriveClient) OnTransferComplete(transfer *GoogleDriveFileTransfer) {
	G.fileId = transfer.fileId
	fmt.Printf("[OnTransferComplete]: %s - %s\n", transfer.file.Name(), G.fileId)
	<-G.concurrency
	G.wg.Done()
}

func (G *GoogleDriveClient) OnTransferUpdate(transfer *GoogleDriveFileTransfer, chunk int64) {
	G.mut.Lock()
	defer G.mut.Unlock()
	G.completed += chunk
	fmt.Printf("[OnTransferUpdate]: Name: %s Chunk: %d | Total: %d\n", transfer.file.Name(), chunk, G.completed)
}

func (G *GoogleDriveClient) OnTransferTemporaryError(transfer *GoogleDriveFileTransfer, err error) {
	fmt.Printf("[OnTransferTempError]: %v, %v\n", transfer, err)
	<-G.concurrency
}

//count = -1 for disabling limit
func (G *GoogleDriveClient) ListFilesByParentId(parentId string, name string, count int) ([]*drive.File, error) {
	var files []*drive.File
	pageToken := ""
	query := fmt.Sprintf("'%s' in parents", parentId)
	if name != "" {
		query += fmt.Sprintf(" and name contains '%s'", name)
	}
	for {
		request := G.DriveSrv.Files.List().Q(query).OrderBy("modifiedTime desc").SupportsAllDrives(true).IncludeTeamDriveItems(true).PageSize(1000).
			Fields("nextPageToken,files(id, name,size, mimeType)")
		if pageToken != "" {
			request = request.PageToken(pageToken)
		}
		res, err := request.Do()
		if err != nil {
			return files, err
		}
		for _, f := range res.Files {
			if count != -1 && len(files) == count {
				return files, nil
			}
			files = append(files, f)
		}
		pageToken = res.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return files, nil
}

func (G *GoogleDriveClient) DownloadDir(dir *drive.File, localDir string) error {
	q := NewQueue()
	dirValue := NewDirValue(dir.Id, localDir)
	q.Enqueue(dirValue)
	for !q.IsEmpty() {
		if G.isCancelled {
			return errors.New("Cancelled by user.")
		}
		dirItem := q.Deque()
		files, err := G.ListFilesByParentId(dirItem.Src, "", -1)
		if err != nil {
			return nil
		}
		for _, file := range files {
			if G.isCancelled {
				return errors.New("Cancelled by user.")
			}
			absPath := filepath.Join(dirItem.Des, file.Name)
			if file.MimeType == "application/vnd.google-apps.folder" {
				err = os.MkdirAll(absPath, 0755)
				if err != nil {
					return err
				}
				v := NewDirValue(file.Id, absPath)
				q.Enqueue(v)
			} else {
				err = G.HandleDownloadFile(file, absPath)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (G *GoogleDriveClient) UploadDir(dir string, parentId string) error {
	q := NewQueue()
	dirValue := NewDirValue(dir, parentId)
	q.Enqueue(dirValue)
	for !q.IsEmpty() {
		if G.isCancelled {
			return errors.New("Cancelled by user.")
		}
		dirItem := q.Deque()
		files, err := ioutil.ReadDir(dirItem.Src)
		if err != nil {
			return err
		}
		for _, file := range files {
			if G.isCancelled {
				return errors.New("Cancelled by user.")
			}
			absPath := filepath.Join(dirItem.Src, file.Name())
			if file.IsDir() {
				dirV, err := G.CreateDir(filepath.Base(file.Name()), dirItem.Des)
				if err != nil {
					return err
				}
				v := NewDirValue(absPath, dirV.Id)
				q.Enqueue(v)
			} else {
				err := G.HandleUploadFile(absPath, dirItem.Des, func(f *drive.File) {})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (G *GoogleDriveClient) HandleDownloadFile(file *drive.File, localDir string) error {
	service, err := G.GetDriveService()
	if err != nil {
		return err
	}
	transfer := NewGoogleDriveFileTransfer(service, G, nil)
	G.concurrency <- 1
	G.wg.Add(1)
	go transfer.Download(file, localDir, 0)
	G.currentTransferQueue = append(G.currentTransferQueue, transfer)
	return nil
}

func (G *GoogleDriveClient) HandleUploadFile(path string, parentId string, cb func(*drive.File)) error {
	service, err := G.GetDriveService()
	if err != nil {
		return err
	}
	transfer := NewGoogleDriveFileTransfer(service, G, cb)
	G.concurrency <- 1
	G.wg.Add(1)
	go transfer.Upload(path, parentId, 0)
	G.currentTransferQueue = append(G.currentTransferQueue, transfer)
	return nil
}

func (G *GoogleDriveClient) Cancel() {
	for _, tr := range G.currentTransferQueue {
		tr.Cancel()
	}
}

func (G *GoogleDriveClient) GetFileMetadata(fileId string) (*drive.File, error) {
	file, err := G.DriveSrv.Files.Get(fileId).Fields("name,mimeType,size,id,md5Checksum").SupportsAllDrives(true).Do()
	return file, err
}

func (G *GoogleDriveClient) Download(fileId string, localDir string) error {
	G.listener.OnTransferStart(G)
	meta, err := G.GetFileMetadata(fileId)
	var outPath string
	if err != nil {
		G.listener.OnTransferError(G, err)
		return nil
	}
	if meta.MimeType == "application/vnd.google-apps.folder" {
		outPath = filepath.Join(localDir, meta.Name)
		err = os.MkdirAll(outPath, 0755)
		if err != nil {
			G.listener.OnTransferError(G, err)
			return nil
		}
		err = G.DownloadDir(meta, outPath)
		if err != nil {
			G.listener.OnTransferError(G, err)
			return nil
		}
	} else {
		err = G.HandleDownloadFile(meta, localDir)
		if err != nil {
			G.listener.OnTransferError(G, err)
			return nil
		}
	}
	G.wg.Wait()
	G.listener.OnTransferComplete(G, outPath)
	return nil
}

func (G *GoogleDriveClient) Upload(path string, parentId string) error {
	G.listener.OnTransferStart(G)
	stat, err := os.Stat(path)
	if err != nil {
		G.listener.OnTransferError(G, err)
		return err
	}
	var fileId string
	if stat.IsDir() {
		dir, err := G.CreateDir(filepath.Base(path), parentId)
		if err != nil {
			G.listener.OnTransferError(G, err)
			return err
		}
		fileId = dir.Id
		err = G.UploadDir(path, dir.Id)
		if err != nil {
			G.listener.OnTransferError(G, err)
			return nil
		}
	} else {
		err = G.HandleUploadFile(path, parentId, func(f *drive.File) {
			fileId = f.Id
		})
		if err != nil {
			G.listener.OnTransferError(G, err)
			return nil
		}
	}
	G.wg.Wait()
	G.listener.OnTransferComplete(G, fileId)
	return nil
}
