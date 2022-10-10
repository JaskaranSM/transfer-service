package main

import (
	"time"

	"github.com/jaskaranSM/transfer-service/manager"
)

func main() {
	client := manager.NewGoogleDriveManager()
	// _, err := manager.AddUpload(&AddUploadOpts{
	// 	Path:               "D:/Evan Call - APPARE-RANMAN! ORIGINAL SOUNDTRACK Appare Tokidoki Kosame MP3/",
	// 	ParentId:           parent,
	// 	Size:               0,
	// 	Concurrency:        10,
	// 	CleanAfterComplete: false,
	// })
	_, err := client.AddDownload(&manager.AddDownloadOpts{
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
