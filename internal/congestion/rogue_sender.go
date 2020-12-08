package congestion

import (
	"math"
	"time"

	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/utils"
)

type rogueSender struct {
	budgetAtLastSent int64
	lastSentTime     time.Time
	maxBurstSize     int64
	bandwidth        Bandwidth
	clock            Clock
}

func NewRogueSender(clock Clock, bandwidth Bandwidth) *rogueSender {
	c := &rogueSender{
		clock:     clock,
		bandwidth: bandwidth,
	}
	c.maxBurstSize = utils.MaxInt64(
		(protocol.MinPacingDelay+protocol.TimerGranularity).Nanoseconds()*int64(bandwidth)/8/1e9,
		int64(maxBurstSize),
	)
	c.budgetAtLastSent = c.maxBurstSize
	c.lastSentTime = clock.Now()
	return c
}

func (c *rogueSender) TimeUntilSend(_ protocol.ByteCount) time.Time {
	deficit := int64(maxDatagramSize) - c.budgetAtLastSent
	if deficit <= 0 {
		return time.Time{}
	}
	delta := time.Duration(math.Ceil(1e9*float64(deficit)/float64(c.bandwidth/8))) * time.Nanosecond
	delta = utils.MaxDuration(delta, protocol.MinPacingDelay)
	return c.lastSentTime.Add(delta)
}

func (c *rogueSender) HasPacingBudget() bool {
	if c.budgetAtLastSent >= int64(maxDatagramSize) {
		return true
	}
	elapsed := c.clock.Now().Sub(c.lastSentTime)
	return float64(c.budgetAtLastSent)+elapsed.Seconds()*float64(c.bandwidth/8) >= float64(maxDatagramSize)
}

func (c *rogueSender) OnPacketSent(
	sentTime time.Time,
	bytesInFlight protocol.ByteCount,
	packetNumber protocol.PacketNumber,
	bytes protocol.ByteCount,
	isRetransmittable bool,
) {
	c.budgetAtLastSent += int64(sentTime.Sub(c.lastSentTime).Seconds() * float64(c.bandwidth/8))
	c.budgetAtLastSent = utils.MinInt64(c.budgetAtLastSent-int64(bytes), c.maxBurstSize)
	c.lastSentTime = sentTime
}

func (c *rogueSender) CanSend(bytesInFlight protocol.ByteCount) bool {
	return true
}

func (c *rogueSender) MaybeExitSlowStart() {
}

func (c *rogueSender) OnPacketAcked(
	ackedPacketNumber protocol.PacketNumber,
	ackedBytes protocol.ByteCount,
	priorInFlight protocol.ByteCount,
	eventTime time.Time,
) {
}

func (c *rogueSender) OnPacketLost(
	packetNumber protocol.PacketNumber,
	lostBytes protocol.ByteCount,
	priorInFlight protocol.ByteCount,
) {
}

func (c *rogueSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
}

func (c *rogueSender) InRecovery() bool {
	return false
}

func (c *rogueSender) InSlowStart() bool {
	return false
}

func (c *rogueSender) GetCongestionWindow() protocol.ByteCount {
	return 2
}
