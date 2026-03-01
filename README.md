# Transfer Service

A high-performance, concurrent service for transferring files and directories to and from Google Drive. Built with Go and Fiber, this service provides a REST API to manage uploads, downloads, and server-side clones with progress tracking.

## Features

- **Concurrent Transfers**: Leverages goroutines for high-speed, parallel file transfers.
- **REST API**: Simple and intuitive API for initiating and monitoring transfers.
- **Google Drive Operations**:
  - **Upload**: Transfer local files and directories to Google Drive.
  - **Download**: Fetch files and directories from Google Drive to the local filesystem.
  - **Clone**: Perform efficient server-side copies of files and directories within Google Drive.
- **Progress Tracking**: Real-time status updates including transfer speed, completed size, and overall progress via a unique task `gid`.
- **Cancel Transfers**: Ability to cancel any in-progress transfer.
- **Flexible Authentication**: Supports both OAuth 2.0 for user accounts and service accounts for automated workflows.
- **Containerized**: Ready to deploy with Docker and Docker Compose, with multi-architecture images (amd64, arm64) available on GHCR.
- **Directory Support**: Recursively handles entire directories for all transfer operations.

## Getting Started

The recommended way to run the service is using Docker.

### Prerequisites
- Docker and Docker Compose
- Google Drive API credentials.

### Configuration

The service is configured using a `.env` file or environment variables.

1.  **Create a `.env` file** in the same directory where you will run `docker-compose.yaml`.

2.  **Authentication Setup**:

    - **Option 1: Using OAuth 2.0 (Recommended for personal use)**
      - Place your `credentials.json` file from Google Cloud Console in the project root.
      - Set `USE_SA=false` in your `.env` file.
      - On the first run, the service will log an authentication URL. Open it, authorize the app, and paste the received code back into the terminal. A `token.json` file will be created to handle future authentications automatically.

    - **Option 2: Using Service Accounts (Recommended for automation/cloning)**
      - Create a directory named `accounts` in the project root.
      - Place your service account JSON key files inside the `accounts/` directory.
      - Set `USE_SA=true` in your `.env` file.

3.  **Example `.env` file**:
    ```env
    # The port the service will listen on
    APP_PORT=6969

    # Set to true to use Service Accounts, false for OAuth 2.0
    USE_SA=true
    ```

### Running with Docker Compose

Use the following `docker-compose.yaml` configuration. It mounts the necessary local directories for configuration and data into the container.

```yaml
version: "3.9"

services:
  transfer-service:
    image: ghcr.io/jaskaransm/transfer-service:latest
    container_name: transfer-service
    restart: unless-stopped
    ports:
      - "6969:6969"
    volumes:
      # Mount your local data directory to /data inside the container
      - ./data:/data 
      # Mount directory for OAuth credentials and token
      - ./credentials.json:/app/credentials.json
      - ./token.json:/app/token.json
      # Mount directory for Service Account keys
      - ./accounts:/app/accounts
      # Mount .env file for configuration
      - ./.env:/app/.env
```

To start the service, run:
```sh
docker-compose up -d
```

## API Usage

The service exposes a REST API for managing transfers. Each new transfer operation returns a unique `gid` (Group ID) which can be used to track its status.

---

### **Start an Upload**

Upload a local file or directory to a specific Google Drive folder.

`POST /api/v1/upload`

**Request Body:**
```json
{
    "path": "/data/my-local-folder",
    "parent_id": "google_drive_folder_id",
    "concurrency": 10
}
```
*   `path`: Absolute path inside the container to the file/directory to upload.
*   `parent_id`: The ID of the destination folder in Google Drive.
*   `concurrency`: Number of concurrent file streams.

**Response:**
```json
{
    "gid": "aBcDeFgHiJkLmNoP"
}
```

---

### **Start a Download**

Download a file or folder from Google Drive to a local directory.

`POST /api/v1/download`

**Request Body:**
```json
{
    "file_id": "google_drive_file_or_folder_id",
    "local_dir": "/data/downloads",
    "concurrency": 10
}
```
*   `file_id`: The ID of the file/folder to download from Google Drive.
*   `local_dir`: Absolute path inside the container where the content will be saved.
*   `concurrency`: Number of concurrent file streams.

**Response:**
```json
{
    "gid": "qRsTuVwXyZaBcDeF"
}
```

---

### **Start a Clone**

Perform a server-side copy of a file or folder to another location in Google Drive.

`POST /api/v1/clone`

**Request Body:**
```json
{
    "file_id": "source_google_drive_file_id",
    "des_id": "destination_google_drive_folder_id",
    "concurrency": 20
}
```
*   `file_id`: The ID of the file/folder to clone.
*   `des_id`: The ID of the destination folder in Google Drive.
*   `concurrency`: Number of concurrent API calls.

**Response:**
```json
{
    "gid": "gHiJkLmNoPqRsTuV"
}
```

---

### **Get Transfer Status**

Check the status of an ongoing or completed transfer using its `gid`.

`GET /api/v1/transferstatus/:gid`

**Example:** `GET /api/v1/transferstatus/aBcDeFgHiJkLmNoP`

**Response:**
```json
{
    "gid": "aBcDeFgHiJkLmNoP",
    "name": "my-local-folder",
    "total_length": 1073741824,
    "completed_length": 536870912,
    "is_completed": false,
    "is_failed": false,
    "speed": 5242880,
    "transfer_type": "upload",
    "file_id": null,
    "error": null
}
```
*   `speed`: Transfer speed in bytes/sec.
*   `file_id`: The ID of the created Google Drive file/folder (available on completion).

---

### **Cancel a Transfer**

Stop an in-progress transfer.

`POST /api/v1/cancel`

**Request Body:**
```json
{
    "gid": "aBcDeFgHiJkLmNoP"
}
```

**Response:**
```json
{
    "gid": "aBcDeFgHiJkLmNoP"
}
```

---

### **List Files in a Folder**

List the contents of a Google Drive folder.

`POST /api/v1/listfiles`

**Request Body:**
```json
{
    "parent_id": "google_drive_folder_id",
    "name": "optional-name-filter",
    "count": 50
}
```
*   `count`: Set to `-1` to disable the limit.

**Response:**
```json
{
    "files": [
        {
            "id": "...",
            "mimeType": "...",
            "name": "...",
            "size": "..."
        }
    ]
}
```

---

### **Get File Metadata**

Retrieve metadata for a specific file or folder in Google Drive.

`GET /api/v1/filemetadata/:fileId`

**Example:** `GET /api/v1/filemetadata/some_google_drive_file_id`

**Response:**
```json
{
    "file": {
        "id": "...",
        "md5Checksum": "...",
        "mimeType": "...",
        "name": "...",
        "size": "..."
    }
}