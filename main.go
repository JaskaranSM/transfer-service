package main

import (
	"time"
)

func main() {
	manager := NewGoogleDriveManager()
	// _, err := manager.AddUpload(&AddUploadOpts{
	// 	Path:               "D:/Evan Call - APPARE-RANMAN! ORIGINAL SOUNDTRACK Appare Tokidoki Kosame MP3/",
	// 	ParentId:           parent,
	// 	Size:               0,
	// 	Concurrency:        10,
	// 	CleanAfterComplete: false,
	// })
	_, err := manager.AddDownload(&AddDownloadOpts{
		FileId:      "1MqwvxpG-mcKvLwayk2WMwnc_zdiQWtOx",
		LocalDir:    ".",
		Size:        0,
		Concurrency: 10,
	})
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(1)
	}

}
