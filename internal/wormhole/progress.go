package wormhole

import (
	"context"
	"io"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ProgressData represents the progress state payload sent to the frontend.
type ProgressData struct {
	Role     string  `json:"role"`     // "send" or "receive"
	Bytes    int64   `json:"bytes"`    // Bytes transferred so far
	Total    int64   `json:"total"`    // Total file size
	Percent  float64 `json:"percent"`  // Percentage of completion (0.0 to 100.0)
	Speed    float64 `json:"speed"`    // Real-time speed (bytes/sec)
	ETA      float64 `json:"eta"`      // Estimated Time of Arrival (seconds remaining)
	Finished bool    `json:"finished"` // Whether the transfer is completed
}

// ProgressReadSeeker wraps an io.ReadSeeker to intercept reads and seek operations, reporting progress.
type ProgressReadSeeker struct {
	RS        io.ReadSeeker
	Total     int64
	ReadBytes int64
	Ctx       context.Context
	WailsCtx  context.Context
	Role      string
	StartTime time.Time
	LastEmit  time.Time
}

// NewProgressReadSeeker creates a new progress tracking reader.
func NewProgressReadSeeker(rs io.ReadSeeker, total int64, ctx context.Context, wailsCtx context.Context, role string) *ProgressReadSeeker {
	now := time.Now()
	return &ProgressReadSeeker{
		RS:        rs,
		Total:     total,
		Ctx:       ctx,
		WailsCtx:  wailsCtx,
		Role:      role,
		StartTime: now,
		LastEmit:  now,
	}
}

func (prs *ProgressReadSeeker) Read(p []byte) (n int, err error) {
	// Support context cancellation
	select {
	case <-prs.Ctx.Done():
		return 0, prs.Ctx.Err()
	default:
	}

	n, err = prs.RS.Read(p)
	if n > 0 {
		prs.ReadBytes += int64(n)
		prs.emit(false)
	}
	return n, err
}

func (prs *ProgressReadSeeker) Seek(offset int64, whence int) (int64, error) {
	pos, err := prs.RS.Seek(offset, whence)
	if err == nil {
		prs.ReadBytes = pos
	}
	return pos, err
}

func (prs *ProgressReadSeeker) emit(force bool) {
	now := time.Now()
	elapsed := now.Sub(prs.StartTime).Seconds()

	// Rate-limit emits to once every 150ms unless it is a forced final update
	if !force && now.Sub(prs.LastEmit) < 150*time.Millisecond && prs.ReadBytes < prs.Total {
		return
	}
	prs.LastEmit = now

	var speed float64
	if elapsed > 0.05 {
		speed = float64(prs.ReadBytes) / elapsed
	}

	var eta float64
	if speed > 0 && prs.Total > prs.ReadBytes {
		eta = float64(prs.Total-prs.ReadBytes) / speed
	}

	percent := 0.0
	if prs.Total > 0 {
		percent = (float64(prs.ReadBytes) / float64(prs.Total)) * 100.0
	}
	if percent > 100 {
		percent = 100
	}

	finished := prs.ReadBytes >= prs.Total || force

	runtime.EventsEmit(prs.WailsCtx, "transfer:progress", ProgressData{
		Role:     prs.Role,
		Bytes:    prs.ReadBytes,
		Total:    prs.Total,
		Percent:  percent,
		Speed:    speed,
		ETA:      eta,
		Finished: finished,
	})
}

// ForceComplete emits the final completed event
func (prs *ProgressReadSeeker) ForceComplete() {
	prs.ReadBytes = prs.Total
	prs.emit(true)
}

// ProgressWriter wraps an io.Writer to track write progress.
type ProgressWriter struct {
	W            io.Writer
	Total        int64
	WrittenBytes int64
	Ctx          context.Context
	WailsCtx     context.Context
	Role         string
	StartTime    time.Time
	LastEmit     time.Time
}

// NewProgressWriter creates a new progress tracking writer.
func NewProgressWriter(w io.Writer, total int64, ctx context.Context, wailsCtx context.Context, role string) *ProgressWriter {
	now := time.Now()
	return &ProgressWriter{
		W:         w,
		Total:     total,
		Ctx:       ctx,
		WailsCtx:  wailsCtx,
		Role:      role,
		StartTime: now,
		LastEmit:  now,
	}
}

func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	// Support context cancellation
	select {
	case <-pw.Ctx.Done():
		return 0, pw.Ctx.Err()
	default:
	}

	n, err = pw.W.Write(p)
	if n > 0 {
		pw.WrittenBytes += int64(n)
		pw.emit(false)
	}
	return n, err
}

func (pw *ProgressWriter) emit(force bool) {
	now := time.Now()
	elapsed := now.Sub(pw.StartTime).Seconds()

	if !force && now.Sub(pw.LastEmit) < 150*time.Millisecond && pw.WrittenBytes < pw.Total {
		return
	}
	pw.LastEmit = now

	var speed float64
	if elapsed > 0.05 {
		speed = float64(pw.WrittenBytes) / elapsed
	}

	var eta float64
	if speed > 0 && pw.Total > pw.WrittenBytes {
		eta = float64(pw.Total-pw.WrittenBytes) / speed
	}

	percent := 0.0
	if pw.Total > 0 {
		percent = (float64(pw.WrittenBytes) / float64(pw.Total)) * 100.0
	}
	if percent > 100 {
		percent = 100
	}

	finished := pw.WrittenBytes >= pw.Total || force

	runtime.EventsEmit(pw.WailsCtx, "transfer:progress", ProgressData{
		Role:     pw.Role,
		Bytes:    pw.WrittenBytes,
		Total:    pw.Total,
		Percent:  percent,
		Speed:    speed,
		ETA:      eta,
		Finished: finished,
	})
}

// ForceComplete emits the final completed event
func (pw *ProgressWriter) ForceComplete() {
	pw.WrittenBytes = pw.Total
	pw.emit(true)
}
