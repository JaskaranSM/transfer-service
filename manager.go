package main

import (
	"fmt"
	"os"
	"time"
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
	client                         *GoogleDriveClient
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

func (g *GoogleDriveTransferStatus) SetClient(client *GoogleDriveClient) {
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

func (g *GoogleDriveTransferStatus) OnTransferComplete(client *GoogleDriveClient, fileId string) {
	g.isCompleted = true
	g.StopSpeedObserver()
	if g.isUpload {
		fmt.Printf("[onUploadComplete]: %s\n", fileId)
	} else {
		fmt.Printf("[onDownloadComplete]: %s\n", fileId)
	}

	g.onTransferCompleteUserCallback()
}

func (g *GoogleDriveTransferStatus) OnTransferStart(client *GoogleDriveClient) {
	g.StartSpeedObserver()
	if g.isUpload {
		fmt.Printf("[onUploadStart]: %v\n", client)
	} else {
		fmt.Printf("[onDownloadStart]: %v\n", client)
	}
}

func (g *GoogleDriveTransferStatus) OnTransferError(client *GoogleDriveClient, err error) {
	g.isFailed = true
	g.StopSpeedObserver()
	if g.isUpload {
		fmt.Printf("[onUploadError]: %v\n", err)
	} else {
		fmt.Printf("[onDownloadError]: %v\n", err)
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
	if opts.Gid == "" {
		opts.Gid = "asdjajfjwrfwe"
	}
	status := NewGoogleDriveTransferStatus(opts.Gid, false, opts.FileId, false, func() {
		os.Exit(0)
	})
	client := NewGoogleDriveClient(opts.Concurrency, opts.Size, status)
	status.SetClient(client)
	g.queue[status.gid] = status
	err := client.Authorize()
	if err != nil {
		return opts.Gid, err
	}
	go client.Download(opts.FileId, opts.LocalDir)
	return opts.Gid, nil
}

func (g *GoogleDriveManager) AddUpload(opts *AddUploadOpts) (string, error) {
	if opts.Gid == "" {
		opts.Gid = "asdjajfjwrfwe"
	}
	status := NewGoogleDriveTransferStatus(opts.Gid, true, opts.Path, opts.CleanAfterComplete, func() {
		os.Exit(0)
	})
	client := NewGoogleDriveClient(opts.Concurrency, opts.Size, status)
	status.SetClient(client)
	g.queue[status.gid] = status
	err := client.Authorize()
	if err != nil {
		return opts.Gid, err
	}
	go client.Upload(opts.Path, opts.ParentId)
	return opts.Gid, nil
}
