# Videoservice

Videoservice is a video management microservice, where the user can upload their videos, browse their videos, stream the video content, and pull video frames and thumbnails.

## Design note
The implementation of video streaming relies on the browser support, so only MP4, WebM, and Ogg formats are supported. The microservice uses [ffmpeg](https://ffmpeg.org/) to extract video frames and resize them. When creating a thumbnail `ffmpeg` is also used for padding the frame to fit into a specified rectangle (`ThumbnailWidth`, `ThumbnailHeight`). The `ffmpeg` processes are running concurently (up to `MaxProcesses`) to serve multiple users at once. Video files are stored in the local file system. Any attempts to upload a file with existing name are rejected.

## Prerequisites
Make sure [ffmpeg](https://ffmpeg.org/) is installed in the system beforehand.

## Configuration
The configuration is stored in `toml` files
* `config-local.toml` for running locally
* `config-prod.toml` for running in a production environment

The configuration file is picked automatically according to the value of the `CONTAINER_ENVIRONMENT` environment variable. If it is not defined, the local configuration is applied.
```
[API]
ListenPort = 3000         # API listening port

ThumbnailWidth = 160      # Thumbnail rectangle width in pixels
ThumbnailHeight = 128     # Thumbnail rectangle height in pixels

[Framer]
MaxProcesses = 8          # Max number of concurrently running ffmpeg

[Storage]
LocalDirectory = "video"  # Local file system directory name for storing videos
```

# API

## Upload a video
```
POST /upload

<Multi-part MIME-encoded form data>

The post form has to contain a 'fileinfo' parameter representing the name of the video file.
```
Status: `201`

## Stream a video
```
GET /video/{filename}
```
Status: `200`
```
<Multi-part MIME-encoded video stream>
```

## Get a video frame by index
```
GET /video/{filename}/frame/{index}
```
Query parameters
```
width  - resize the frame to the given width in pixels
height - resize the frame to the given height in pixels
```
Status: `200`
```
Content-Type: image/jpeg

<JPEG image>
```

## Get a thumbnail of a video frame by index
```
GET /video/{filename}/frame/{index}/thumbnail
```
Status: `200`
```
Content-Type: image/jpeg

<JPEG image>
```
The frame is resized to fit into a preconfigured rectangle. By default, 128 x 128.

## Get a video list
```
GET /videos
```
**Status**: 200
```
Content-Type: application/json

{
    "files": [
        {
            "name": "big_buck_bunny.mp4",
            "size": 5510872
        },
        {
            "name": "aux.mp4",
            "size": 290215220
        },
    ]
}
```

## TODO
* Support proper object storage for video files, e.g., [Minio](https://github.com/minio/minio).
* Support other video formats.
* Use faster logger.
* Add Swagger doc.
* Comprehensive error checking.
