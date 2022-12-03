package manager

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/jaskaranSM/transfer-service/logging"
	"github.com/jaskaranSM/transfer-service/service/gdrive"
	gdriveconstants "github.com/jaskaranSM/transfer-service/service/gdrive/constants"
	"github.com/jaskaranSM/transfer-service/utils"
)

func NewGoogleDriveTransferStatus(gid string, transferType string, path string, cleanAfterComplete bool, OnTransferComplete func()) *GoogleDriveTransferStatus {
	return &GoogleDriveTransferStatus{
		gid:                            gid,
		transferType:                   transferType,
		path:                           path,
		cleanAfterComplete:             cleanAfterComplete,
		onTransferCompleteUserCallback: OnTransferComplete,
	}
}

type GoogleDriveTransferStatus struct {
	client                         *gdrive.GoogleDriveClient
	isCompleted                    bool
	isFailed                       bool
	isObserverRunning              bool
	speed                          int64
	gid                            string
	cleanAfterComplete             bool
	path                           string
	transferType                   string
	fileID                         string
	err                            error
	onTransferCompleteUserCallback func()
}

func (g *GoogleDriveTransferStatus) SetClient(client *gdrive.GoogleDriveClient) {
	g.client = client
}

func (g *GoogleDriveTransferStatus) cleanup() error {
	if g.cleanAfterComplete {
		return os.RemoveAll(g.path)
	}
	return nil
}

func (g *GoogleDriveTransferStatus) StartSpeedObserver() {
	g.isObserverRunning = true
	go g.SpeedObserver()
}

func (g *GoogleDriveTransferStatus) StopSpeedObserver() {
	g.isObserverRunning = false
}

func (g *GoogleDriveTransferStatus) SpeedObserver() {
	last := g.CompletedLength()
	for g.isObserverRunning {
		now := g.CompletedLength()
		chunk := now - last
		g.speed = chunk
		last = now
		time.Sleep(1 * time.Second)
	}
}

func (g *GoogleDriveTransferStatus) OnTransferComplete(client *gdrive.GoogleDriveClient, fileId string) {
	logger := logging.GetLogger()
	g.fileID = fileId
	g.isCompleted = true
	g.StopSpeedObserver()
	logger.Debug(fmt.Sprintf("on %s complete: ", g.transferType), zap.String("fileID", fileId))
	g.onTransferCompleteUserCallback()
}

func (g *GoogleDriveTransferStatus) OnTransferStart(client *gdrive.GoogleDriveClient) {
	logger := logging.GetLogger()
	g.StartSpeedObserver()
	logger.Debug(fmt.Sprintf("on %s start: ", g.transferType))
}

func (g *GoogleDriveTransferStatus) OnTransferError(client *gdrive.GoogleDriveClient, err error) {
	logger := logging.GetLogger()
	g.isFailed = true
	g.err = err
	g.StopSpeedObserver()
	logger.Debug(fmt.Sprintf("on %s Error: ", g.transferType), zap.Error(err))
}

func (g *GoogleDriveTransferStatus) GetTransferType() string {
	return g.transferType
}

func (g *GoogleDriveTransferStatus) GetFileID() string {
	return g.fileID
}

func (g *GoogleDriveTransferStatus) CompletedLength() int64 {
	return g.client.CompletedLength()
}

func (g *GoogleDriveTransferStatus) TotalLength() int64 {
	return g.client.TotalLength()
}

func (g *GoogleDriveTransferStatus) Speed() int64 {
	return g.speed
}

func (g *GoogleDriveTransferStatus) IsCompleted() bool {
	return g.isCompleted
}

func (g *GoogleDriveTransferStatus) IsFailed() bool {
	return g.isFailed
}

func (g *GoogleDriveTransferStatus) GetFailureError() error {
	return g.err
}

func (g *GoogleDriveTransferStatus) Name() string {
	return g.client.Name
}

func (g *GoogleDriveTransferStatus) Cancel() {
	g.client.Cancel()
}

type AddUploadOpts struct {
	Path                     string
	ParentId                 string
	Gid                      string
	CleanAfterComplete       bool
	Size                     int64
	Concurrency              int
	OnUploadCompleteCallback func()
}

type AddDownloadOpts struct {
	FileId                     string
	LocalDir                   string
	Gid                        string
	Size                       int64
	Concurrency                int
	OnDownloadCompleteCallback func()
}

type AddCloneOpts struct {
	FileId                  string
	DesId                   string
	Gid                     string
	Size                    int64
	Concurrency             int
	OnCloneCompleteCallback func()
}

func NewGoogleDriveManager() *GoogleDriveManager {
	return &GoogleDriveManager{
		queue: make(map[string]*GoogleDriveTransferStatus),
	}
}

type GoogleDriveManager struct {
	queue map[string]*GoogleDriveTransferStatus
}

func (g *GoogleDriveManager) GetTransferStatusByGid(gid string) *GoogleDriveTransferStatus {
	return g.queue[gid]
}

func (g *GoogleDriveManager) AddDownload(opts *AddDownloadOpts) (string, error) {
	if opts.Gid == "" {
		opts.Gid = utils.RandString(16)
	}

	status := NewGoogleDriveTransferStatus(opts.Gid, gdriveconstants.TransferTypeDownloading, opts.FileId, false, func() {
	})

	client := gdrive.NewGoogleDriveClient(opts.Concurrency, opts.Size, status)
	status.SetClient(client)
	g.queue[status.gid] = status
	err := client.Authorize()
	if err != nil {
		return opts.Gid, err
	}
	go func() {
		err = client.Download(opts.FileId, opts.LocalDir)
		if err != nil {
			return
		}
	}()

	return opts.Gid, nil
}

func (g *GoogleDriveManager) AddClone(opts *AddCloneOpts) (string, error) {
	logger := logging.GetLogger()
	if opts.Gid == "" {
		opts.Gid = utils.RandString(16)
	}
	status := NewGoogleDriveTransferStatus(opts.Gid, gdriveconstants.TransferTypeCloning, opts.FileId, false, func() {
	})
	client := gdrive.NewGoogleDriveClient(opts.Concurrency, opts.Size, status)
	status.SetClient(client)
	g.queue[status.gid] = status
	err := client.Authorize()
	if err != nil {
		return opts.Gid, err
	}
	go func() {
		err := client.Clone(opts.FileId, opts.DesId)
		if err != nil {
			logger.Error("Error while uploading file", zap.Error(err))
		}
	}()
	return opts.Gid, nil
}

func (g *GoogleDriveManager) AddUpload(opts *AddUploadOpts) (string, error) {
	logger := logging.GetLogger()
	if opts.Gid == "" {
		opts.Gid = utils.RandString(16)
	}
	status := NewGoogleDriveTransferStatus(opts.Gid, gdriveconstants.TransferTypeUploading, opts.Path, opts.CleanAfterComplete, func() {
	})
	if opts.Size == 0 {
		size, err := utils.GetPathSize(opts.Path)
		if err != nil {
			logger.Error("Could not get path size", zap.Error(err))
		}
		opts.Size = size
	}
	client := gdrive.NewGoogleDriveClient(opts.Concurrency, opts.Size, status)
	status.SetClient(client)
	g.queue[status.gid] = status
	err := client.Authorize()
	if err != nil {
		return opts.Gid, err
	}
	go func() {
		err := client.Upload(opts.Path, opts.ParentId)
		if err != nil {
			logger.Error("Error while uploading file", zap.Error(err))
		}
	}()
	return opts.Gid, nil
}
