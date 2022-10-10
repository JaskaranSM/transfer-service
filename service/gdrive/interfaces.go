package gdrive

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
