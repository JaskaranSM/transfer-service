package manager

import (
	"os"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/jaskaranSM/transfer-service/logging"
	"github.com/jaskaranSM/transfer-service/service/gdrive"
)

func NewGoogleDriveTransferStatus(gid string, isUpload bool, path string, cleanAfterComplete bool, OnTransferComplete func()) *GoogleDriveTransferStatus {
	return &GoogleDriveTransferStatus{
		gid:                            gid,
		isUpload:                       isUpload,
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
	isUpload                       bool
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
	g.isCompleted = true
	g.StopSpeedObserver()
	if g.isUpload {
		logger.Debug("on upload complete: ", zap.String("fileID", fileId))
	} else {
		logger.Debug("on download complete: ", zap.String("fileID", fileId))
	}

	g.onTransferCompleteUserCallback()
}

func (g *GoogleDriveTransferStatus) OnTransferStart(client *gdrive.GoogleDriveClient) {
	logger := logging.GetLogger()
	g.StartSpeedObserver()
	if g.isUpload {
		logger.Debug("on upload start")
	} else {
		logger.Debug("on download start")
	}
}

func (g *GoogleDriveTransferStatus) OnTransferError(client *gdrive.GoogleDriveClient, err error) {
	logger := logging.GetLogger()
	g.isFailed = true
	g.StopSpeedObserver()
	if g.isUpload {
		logger.Debug("on uploader error", zap.Error(err))
	} else {
		logger.Debug("on download error", zap.Error(err))
	}
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
	logger := logging.GetLogger()
	if opts.Gid == "" {
		gid, err := uuid.NewUUID()
		if err != nil {
			logger.Error("Could not create new UUID", zap.Error(err))
			return "", err
		}
		opts.Gid = gid.String()
	}

	status := NewGoogleDriveTransferStatus(opts.Gid, false, opts.FileId, false, func() {
		os.Exit(0)
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

func (g *GoogleDriveManager) AddUpload(opts *AddUploadOpts) (string, error) {
	logger := logging.GetLogger()
	if opts.Gid == "" {
		gid, err := uuid.NewUUID()
		if err != nil {
			logger.Error("Could not create new UUID", zap.Error(err))
			return "", err
		}
		opts.Gid = gid.String()
	}
	status := NewGoogleDriveTransferStatus(opts.Gid, true, opts.Path, opts.CleanAfterComplete, func() {
		os.Exit(0)
	})
	client := gdrive.NewGoogleDriveClient(opts.Concurrency, opts.Size, status)
	status.SetClient(client)
	g.queue[status.gid] = status
	err := client.Authorize()
	if err != nil {
		return opts.Gid, err
	}
	go client.Upload(opts.Path, opts.ParentId)
	return opts.Gid, nil
}
