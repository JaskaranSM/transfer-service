package gdrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/jaskaranSM/transfer-service/config"
	"github.com/jaskaranSM/transfer-service/logging"
	"github.com/jaskaranSM/transfer-service/utils"
)

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
	completedFiles       int
	callbackFired        bool
	fileId               string
	wg                   sync.WaitGroup
	DriveSrv             *drive.Service
	Name                 string
}

func (gd *GoogleDriveClient) GetDriveService() (srv *drive.Service, err error) {
	cfg := config.Get()
	logger := logging.GetLogger()

	client, err := gd.getAuthorizedHTTPClient(cfg.UseSA)
	if err != nil {
		logger.Error("Could not get authorized HTTP client", zap.Error(err),
			zap.Bool("UseSA", cfg.UseSA),
		)
		return
	}

	srv, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		logger.Error("Could not create new google drive service", zap.Error(err))
		return
	}
	return
}

func (gd *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	logger := logging.GetLogger()

	logger.Info(fmt.Sprintf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL),
	)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		logger.Fatal("Unable to read authorization code %v", zap.Error(err))
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		logger.Error("Could not exchange config", zap.Error(err))
	}
	return tok
}

func (gd *GoogleDriveClient) tokenFromFile(file string) (*oauth2.Token, error) {
	logger := logging.GetLogger()
	f, err := os.Open(file)
	if err != nil {
		logger.Error("Could not open file", zap.Error(err),
			zap.String("file path", file),
		)
		return nil, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			logger.Error("Could not close os file handle", zap.Error(err))
			return
		}
	}(f)

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	if err != nil {
		logger.Error("Could not decode token json", zap.Error(err),
			zap.String("file path", file),
		)
	}
	return tok, err
}

// Saves a token to a file path.
func (gd *GoogleDriveClient) saveToken(path string, token *oauth2.Token) {
	logger := logging.GetLogger()
	logger.Info("Saving credential file", zap.String("file path", path))

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Error("Could not open file", zap.Error(err),
			zap.String("file path", path),
		)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			logger.Error("Could not close os file handle", zap.Error(err))
			return
		}
	}(f)

	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		logger.Error("Could not encode token json", zap.Error(err),
			zap.String("file path", path),
		)
		return
	}
}

func (gd *GoogleDriveClient) CreateDir(name string, parentId string) (*drive.File, error) {
	logger := logging.GetLogger()
	logger.Debug("CreateDir: ", zap.String("name", name), zap.String("parentId", parentId))
	d := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentId},
	}
	file, err := gd.DriveSrv.Files.Create(d).SupportsAllDrives(true).Do()
	if err != nil {
		logger.Error("Could not create dir", zap.Error(err),
			zap.String("file path", name),
			zap.String("parentId", parentId),
		)
		return nil, fmt.Errorf("CreateDir: %v", err)
	}

	return file, nil
}

func (gd *GoogleDriveClient) OnTransferError(_ *GoogleDriveFileTransfer, err error) {
	logger := logging.GetLogger()
	logger.Error("Error on Transfer", zap.Error(err))

	<-gd.concurrency
	gd.wg.Done()
	if gd.callbackFired {
		logger.Debug("Already callback fired")
		return
	}
	gd.callbackFired = true
	gd.listener.OnTransferError(gd, err)
}

func (gd *GoogleDriveClient) OnTransferStart(transfer *GoogleDriveFileTransfer) {
	logger := logging.GetLogger()
	logger.Debug("Starting Transfer")
}

func (gd *GoogleDriveClient) OnTransferComplete(transfer *GoogleDriveFileTransfer) {
	gd.completedFiles += 1
	logger := logging.GetLogger()
	gd.fileId = transfer.fileId
	logger.Debug("Transfer Completed",
		zap.String("File_ID", gd.fileId),
		zap.Int("CompletedFiles", gd.completedFiles),
	)

	<-gd.concurrency
	gd.wg.Done()
}

func (gd *GoogleDriveClient) OnTransferUpdate(transfer *GoogleDriveFileTransfer, chunk int64) {
	logger := logging.GetLogger()
	gd.mut.Lock()
	defer gd.mut.Unlock()

	gd.completed += chunk
	logger.Debug("Transfer Updated",
		zap.Int64("file chunk", chunk),
		zap.Int64("Total completed", gd.completed),
	)
}

func (gd *GoogleDriveClient) OnTransferTemporaryError(_ *GoogleDriveFileTransfer, err error) {
	logger := logging.GetLogger()
	logger.Debug("Temporary Error ", zap.Error(err))
	<-gd.concurrency
}

// ListFilesByParentId count = -1 for disabling limit
func (gd *GoogleDriveClient) ListFilesByParentId(parentId string, name string, count int) ([]*drive.File, error) {
	logger := logging.GetLogger()
	var files []*drive.File
	pageToken := ""
	query := fmt.Sprintf("'%s' in parents", parentId)
	if name != "" {
		query += fmt.Sprintf(" and name contains '%s'", name)
	}
	for {
		logger.Debug("Listing files in folder",
			zap.String("query", query),
			zap.String("page token", pageToken),
		)

		request := gd.DriveSrv.Files.List().Q(query).OrderBy("modifiedTime desc").SupportsAllDrives(true).IncludeTeamDriveItems(true).PageSize(1000).
			Fields("nextPageToken,files(id, name, size, mimeType)")

		if pageToken != "" {
			request = request.PageToken(pageToken)
		}

		res, err := request.Do()
		if err != nil {
			logger.Error("Error while doing a request",
				zap.Error(err),
			)
			return files, err
		}

		for _, f := range res.Files {
			if count != -1 && len(files) == count {
				logger.Debug("Returning since files == count")
				return files, nil
			}
			files = append(files, f)
		}
		pageToken = res.NextPageToken
		if pageToken == "" {
			logger.Debug("Page token empty")
			break
		}
	}
	return files, nil
}

func (gd *GoogleDriveClient) HandleCloneFile(file *drive.File, desId string, cb func(*drive.File)) error {
	service, err := gd.GetDriveService()
	if err != nil {
		return err
	}
	transfer := NewGoogleDriveFileTransfer(service, gd, cb)
	gd.concurrency <- 1
	gd.wg.Add(1)
	go transfer.Clone(file, desId, 0)
	gd.currentTransferQueue = append(gd.currentTransferQueue, transfer)
	return nil
}

func (gd *GoogleDriveClient) CloneDir(dir *drive.File, parentId string) error {
	logger := logging.GetLogger()
	q := utils.NewQueue()
	dirValue := utils.NewDirValue(dir.Id, parentId)
	q.Enqueue(dirValue)
	for !q.IsEmpty() {
		if gd.isCancelled {
			return errors.New("cancelled by user")
		}
		dirItem := q.Deque()
		files, err := gd.ListFilesByParentId(dirItem.Src, "", -1)
		if err != nil {
			logger.Error("Error while listing gdrive directory contents", zap.Error(err), zap.String("src", dirItem.Src))
			return err
		}

		for _, file := range files {
			if gd.isCancelled {
				return errors.New("cancelled by user")
			}
			if file.MimeType == "application/vnd.google-apps.folder" {
				newDir, err := gd.CreateDir(file.Name, dirItem.Des)
				if err != nil {
					return err
				}
				q.Enqueue(utils.NewDirValue(file.Id, newDir.Id))
			} else {
				logger.Info(file.Id)
				err := gd.HandleCloneFile(file, dirItem.Des, func(f *drive.File) {})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (gd *GoogleDriveClient) DownloadDir(dir *drive.File, localDir string) error {
	logger := logging.GetLogger()
	q := utils.NewQueue()
	dirValue := utils.NewDirValue(dir.Id, localDir)
	q.Enqueue(dirValue)
	for !q.IsEmpty() {
		if gd.isCancelled {
			return errors.New("cancelled by user")
		}
		dirItem := q.Deque()
		files, err := gd.ListFilesByParentId(dirItem.Src, "", -1)
		if err != nil {
			return err
		}
		for _, file := range files {
			if gd.isCancelled {
				return errors.New("cancelled by user")
			}
			absPath := filepath.Join(dirItem.Des, file.Name)
			if file.MimeType == "application/vnd.google-apps.folder" {
				err = os.MkdirAll(absPath, 0755)
				if err != nil {
					logger.Error("Error while creating directories ", zap.Error(err),
						zap.String("Absolute Path", absPath),
					)
					return err
				}
				v := utils.NewDirValue(file.Id, absPath)
				q.Enqueue(v)
			} else {
				err = gd.HandleDownloadFile(file, dirItem.Des)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (gd *GoogleDriveClient) UploadDir(dir string, parentId string) error {
	logger := logging.GetLogger()
	q := utils.NewQueue()
	dirValue := utils.NewDirValue(dir, parentId)
	q.Enqueue(dirValue)
	for !q.IsEmpty() {
		if gd.isCancelled {
			return errors.New("cancelled by user")
		}
		dirItem := q.Deque()
		files, err := os.ReadDir(dirItem.Src)
		if err != nil {
			logger.Error("Could not Read directory", zap.Error(err),
				zap.String("source directory", dirItem.Src),
			)
			return err
		}
		for _, file := range files {
			if gd.isCancelled {
				return errors.New("cancelled by user")
			}
			absPath := filepath.Join(dirItem.Src, file.Name())
			if file.IsDir() {
				var dirV *drive.File
				basePath := filepath.Base(file.Name())
				dirV, err = gd.CreateDir(basePath, dirItem.Des)
				if err != nil {
					return err
				}
				v := utils.NewDirValue(absPath, dirV.Id)
				q.Enqueue(v)
			} else {
				err = gd.HandleUploadFile(absPath, dirItem.Des, func(f *drive.File) {})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (gd *GoogleDriveClient) HandleDownloadFile(file *drive.File, localDir string) error {
	service, err := gd.GetDriveService()
	if err != nil {
		return err
	}
	transfer := NewGoogleDriveFileTransfer(service, gd, nil)
	gd.concurrency <- 1
	gd.wg.Add(1)
	go transfer.Download(file, path.Join(localDir, file.Name), 0)
	gd.currentTransferQueue = append(gd.currentTransferQueue, transfer)
	return nil
}

func (gd *GoogleDriveClient) HandleUploadFile(path string, parentId string, cb func(*drive.File)) error {
	service, err := gd.GetDriveService()
	if err != nil {
		return err
	}
	transfer := NewGoogleDriveFileTransfer(service, gd, cb)
	gd.concurrency <- 1
	gd.wg.Add(1)
	go transfer.Upload(path, parentId, 0)
	gd.currentTransferQueue = append(gd.currentTransferQueue, transfer)
	return nil
}

func (gd *GoogleDriveClient) Cancel() {
	gd.isCancelled = true
	for _, tr := range gd.currentTransferQueue {
		tr.Cancel()
	}
}

func (gd *GoogleDriveClient) GetFileMetadata(fileId string) (*drive.File, error) {
	logger := logging.GetLogger()
	file, err := gd.DriveSrv.Files.Get(fileId).Fields("name,mimeType,size,id,md5Checksum").SupportsAllDrives(true).Do()
	if err != nil {
		logger.Error("Could not get object from file ID", zap.Error(err),
			zap.String("file ID", fileId),
		)
		return nil, fmt.Errorf("GetFileMetadata: %v", err)
	}
	return file, nil
}

func (gd *GoogleDriveClient) Clone(srcId string, desId string) error {
	logger := logging.GetLogger()
	logger.Info("starting clone", zap.String("srcId", srcId), zap.String("desId", desId))
	gd.listener.OnTransferStart(gd)
	meta, err := gd.GetFileMetadata(srcId)
	if err != nil {
		gd.listener.OnTransferError(gd, err)
		return err
	}
	gd.Name = meta.Name
	var fileId string
	if meta.MimeType == "application/vnd.google-apps.folder" {
		newDir, err := gd.CreateDir(meta.Name, desId)
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return err
		}
		fileId = newDir.Id
		err = gd.CloneDir(meta, newDir.Id)
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return err
		}
	} else {
		err = gd.HandleCloneFile(meta, desId, func(f *drive.File) {
			fileId = f.Id
		})
	}
	gd.wg.Wait()
	for _, tr := range gd.currentTransferQueue {
		if tr.isCompleted == false {
			return tr.err
		}
	}
	gd.listener.OnTransferComplete(gd, fileId)
	return nil
}

func (G *GoogleDriveClient) IsDir(file *drive.File) bool {
	return file.MimeType == "application/vnd.google-apps.folder"
}

func (gd *GoogleDriveClient) GetFolderSize(folderId string, size *int64) {
	files, _ := gd.ListFilesByParentId(folderId, "", -1)
	for _, file := range files {
		if gd.isCancelled {
			return
		}
		if file.MimeType == "application/vnd.google-apps.folder" {
			gd.GetFolderSize(file.Id, size)
		} else {
			*size += file.Size
		}
	}
}

func (gd *GoogleDriveClient) Download(fileId string, localDir string) error {
	logger := logging.GetLogger()
	gd.listener.OnTransferStart(gd)
	meta, err := gd.GetFileMetadata(fileId)
	if err != nil {
		gd.listener.OnTransferError(gd, err)
		return nil
	}
	gd.Name = meta.Name
	if gd.total == 0 {
		gd.Name = "getting metadata"
		if gd.IsDir(meta) {
			gd.GetFolderSize(meta.Id, &gd.total)
		} else {
			gd.total = meta.Size
		}
	}
	gd.Name = meta.Name
	var outPath string
	if meta.MimeType == "application/vnd.google-apps.folder" {
		outPath = filepath.Join(localDir, meta.Name)
		err = os.MkdirAll(outPath, 0755)
		if err != nil {
			logger.Error("Could create directories", zap.Error(err),
				zap.String("file path", outPath),
			)
			gd.listener.OnTransferError(gd, err)
			return nil
		}
		err = gd.DownloadDir(meta, outPath)
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return nil
		}
	} else {
		err = gd.HandleDownloadFile(meta, localDir)
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return nil
		}
	}
	gd.wg.Wait()
	for _, tr := range gd.currentTransferQueue {
		if tr.isCompleted == false {
			return tr.err
		}
	}
	gd.listener.OnTransferComplete(gd, gd.fileId)
	return nil
}

func (gd *GoogleDriveClient) Upload(path string, parentId string) error {
	logger := logging.GetLogger()
	gd.Name = filepath.Base(path)
	gd.listener.OnTransferStart(gd)
	stat, err := os.Stat(path)
	if err != nil {
		logger.Error("Could not get stats of file path", zap.Error(err),
			zap.String("file path", path),
		)
		gd.listener.OnTransferError(gd, err)
		return err
	}
	var fileId string
	if stat.IsDir() {
		var dir *drive.File
		dir, err = gd.CreateDir(filepath.Base(path), parentId)
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return err
		}
		fileId = dir.Id
		err = gd.UploadDir(path, dir.Id)
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return nil
		}
	} else {
		err = gd.HandleUploadFile(path, parentId, func(f *drive.File) {
			fileId = f.Id
		})
		if err != nil {
			gd.listener.OnTransferError(gd, err)
			return nil
		}
	}
	gd.wg.Wait()
	for _, tr := range gd.currentTransferQueue {
		if tr.isCompleted == false {
			return tr.err
		}
	}
	gd.listener.OnTransferComplete(gd, fileId)
	return nil
}
