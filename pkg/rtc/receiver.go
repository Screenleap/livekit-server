package rtc

import (
	"context"
	"io"
	"sync"

	"github.com/pion/ion-sfu/pkg/buffer"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"

	"github.com/livekit/livekit-server/pkg/logger"
)

const (
	// TODO: could probably increase this depending on Configuration/memory
	maxChanSize = 1024
)

// A receiver is responsible for pulling from a remoteTrack
type Receiver struct {
	peerId      string
	ctx         context.Context
	cancel      context.CancelFunc
	rtpReceiver *webrtc.RTPReceiver
	track       *webrtc.TrackRemote
	bi          *buffer.Interceptor
	once        sync.Once
	bytesRead   int64
}

func NewReceiver(ctx context.Context, peerId string, rtpReceiver *webrtc.RTPReceiver, bi *buffer.Interceptor) *Receiver {
	ctx, cancel := context.WithCancel(ctx)
	track := rtpReceiver.Track()
	return &Receiver{
		ctx:         ctx,
		cancel:      cancel,
		peerId:      peerId,
		rtpReceiver: rtpReceiver,
		track:       track,
		bi:          bi,
		once:        sync.Once{},
	}
}

func (r *Receiver) PeerId() string {
	return r.peerId
}

func (r *Receiver) TrackId() string {
	return r.track.ID()
}

// starts reading RTP and push to buffer
func (r *Receiver) Start() {
	r.once.Do(func() {
		go r.rtcpWorker()
	})
}

// Close gracefully close the remoteTrack. if the context is canceled
func (r *Receiver) Close() {
	if r.ctx.Err() != nil {
		return
	}
	r.cancel()
}

// PacketBuffer interface, to provide forwarders packets from the buffer
func (r *Receiver) GetBufferedPackets(mediaSSRC uint32, snOffset uint16, tsOffset uint32, sn []uint16) []rtp.Packet {
	if r.bi == nil {
		return nil
	}
	return r.bi.GetBufferedPackets(uint32(r.track.SSRC()), mediaSSRC, snOffset, tsOffset, sn)
}

func (r *Receiver) ReadRTP() (*rtp.Packet, error) {
	return r.track.ReadRTP()
}

// rtcpWorker reads RTCP messages from receiver, notifies buffer
func (r *Receiver) rtcpWorker() {
	for {
		_, err := r.rtpReceiver.ReadRTCP()
		if err == io.ErrClosedPipe || r.ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.GetLogger().Warnw("receiver error reading RTCP",
				"peer", r.peerId,
				"remoteTrack", r.track.SSRC(),
				"err", err,
			)
			continue
		}
	}
}