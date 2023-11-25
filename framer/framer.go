package framer

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type Config struct {
	MaxProcesses int // Max number of concurrently running ffmpeg processes
}

type Framer struct {
	cfg    Config
	queue  chan *Job
	tokens chan interface{}
}

type Job struct {
	filepath  string
	index     int
	width     int
	height    int
	thumbnail bool
	ch        chan *Result
}

type Result struct {
	Data []byte
	Err  error
}

// NewFramer creates a new framer instance
func NewFramer(cfg Config) *Framer {
	if cfg.MaxProcesses <= 0 {
		cfg.MaxProcesses = 16
	}
	return &Framer{cfg: cfg}
}

// EnqueueJob adds a new job of video frame extraction to the queue and returns a channel to wait for a result
func (f *Framer) EnqueueJob(ctx context.Context, filepath string, index int, width int, height int, thumbnail bool) chan *Result {
	job := &Job{
		filepath:  filepath,
		index:     index,
		width:     width,
		height:    height,
		thumbnail: thumbnail,
		ch:        make(chan *Result, 1),
	}
	select {
	case f.queue <- job:
	case <-ctx.Done():
		close(job.ch)
	}
	return job.ch
}

// Run starts processing a queue of requests to extract a frame of a video file
func (f *Framer) Run(ctx context.Context) {
	f.queue = make(chan *Job, 1)
	f.tokens = make(chan interface{}, f.cfg.MaxProcesses)
	var wg sync.WaitGroup

job_watch:
	for {
		select {
		case <-ctx.Done():
			break job_watch

		case job := <-f.queue:
			select {
			case <-ctx.Done():
				break job_watch

			case f.tokens <- struct{}{}: // Wait for an available slot for running an ffmpeg process
				wg.Add(1)
				go func() {
					defer wg.Done()

					data, err := extractFrame(ctx, job.filepath, job.index, job.width, job.height, job.thumbnail)
					job.ch <- &Result{
						Data: data,
						Err:  err,
					}
					close(job.ch)

					<-f.tokens // Release the slot for running an ffmpeg process
				}()
			}
		}
	}

	wg.Wait()
}

// extractFrame runs ffmpeg to extract a single frame of a video, resize it, and pad to fit into a thumbnail rectange
func extractFrame(ctx context.Context, filepath string, index int, width int, height int, thumbnail bool) ([]byte, error) {
	// Reference
	// https://trac.ffmpeg.org/wiki/Scaling
	// https://ffmpeg-user.ffmpeg.narkive.com/hC7wyltO/combine-vf-and-filter-complex-for-the-same-video

	w := width
	if w <= 0 {
		w = -1
	}

	h := height
	if h <= 0 {
		h = -1
	}

	buf := bytes.NewBuffer(nil)

	input := ffmpeg.Input(filepath)
	input.Context = ctx

	stream := input.Filter("select", ffmpeg.Args{fmt.Sprintf("gte(n,%d)", index)})

	if w > 0 || h > 0 {
		stream = stream.Filter("scale", ffmpeg.Args{fmt.Sprintf("%d:%d:force_original_aspect_ratio=decrease", w, h)})

		if thumbnail && w > 0 && h > 0 {
			stream = stream.Filter("pad", ffmpeg.Args{fmt.Sprintf("%d:%d:(ow-iw)/2:(oh-ih)/2", w, h)})
		}
	}

	err := stream.
		Output("pipe:", ffmpeg.KwArgs{
			"vframes": 1,
			"format":  "image2",
			"vcodec":  "mjpeg",
		}).
		WithOutput(buf).
		Silent(true).
		Run()

	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
