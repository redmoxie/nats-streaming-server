// Copyright 2016 Apcera Inc. All rights reserved.

package stores

import (
	"errors"
	"time"

	"github.com/nats-io/gnatsd/server"
	"github.com/nats-io/go-stan/pb"
	"github.com/nats-io/stan-server/spb"
)

const (
	// TypeMemory is the store type name for memory based stores
	TypeMemory = "MEMORY"
	// TypeFile is the store type name for file based stores
	TypeFile = "FILE"
)

const (
	// AllChannels allows to get state for all channels.
	AllChannels = "*"
)

// Errors.
var (
	ErrTooManyChannels = errors.New("too many channels")
	ErrTooManySubs     = errors.New("too many subscriptions per channel")
)

// Noticef logs a notice statement
func Noticef(format string, v ...interface{}) {
	server.Noticef(format, v...)
}

// ChannelLimits defines some limits on the store interface
type ChannelLimits struct {
	// How many channels are allowed.
	MaxChannels int
	// How many messages per channel are allowed.
	MaxNumMsgs int
	// How many bytes (messages payloads) per channel are allowed.
	MaxMsgBytes uint64
	// How old messages on a channel can be before being removed.
	MaxMsgAge time.Duration
	// How many subscriptions per channel are allowed.
	MaxSubs int
}

// DefaultChannelLimits are the channel limits that a Store must
// use when none are specified to the Store constructor.
// Store limits can be changed with the Store.SetChannelLimits() method.
var DefaultChannelLimits = ChannelLimits{
	MaxChannels: 100,
	MaxNumMsgs:  1000000,
	MaxMsgBytes: 1000000 * 1024,
	MaxSubs:     1000,
}

// RecoveredState allows the server to reconstruct its state after a restart.
type RecoveredState struct {
	Info    *spb.ServerInfo
	Clients []*RecoveredClient
	Subs    RecoveredSubscriptions
}

// RecoveredClient represents a recovered Client with ID and Heartbeat Inbox
type RecoveredClient struct {
	ClientID string
	HbInbox  string
}

// RecoveredSubscriptions is a map of recovered subscriptions, keyed by channel name.
type RecoveredSubscriptions map[string][]*RecoveredSubState

// PendingAcks is a map of messages waiting to be acknowledged, keyed by
// message sequence number.
type PendingAcks map[uint64]*pb.MsgProto

// RecoveredSubState represents a recovered Subscription with a map
// of pending messages.
type RecoveredSubState struct {
	Sub     *spb.SubState
	Pending PendingAcks
}

// ChannelStore contains a reference to both Subscription and Message stores.
type ChannelStore struct {
	// UserData allows the user of a ChannelStore to store private data.
	UserData interface{}
	// Subs is the Subscriptions Store.
	Subs SubStore
	// Msgs is the Messages Store.
	Msgs MsgStore
}

// Store is the storage interface for STAN servers.
//
// If an implementation has a Store constructor with ChannelLimits, it should be
// noted that the limits don't apply to any state being recovered, for Store
// implementations supporting recovery.
//
// When calling the method LookupOrCreateChannel(), if the channel does not exist,
// the implementation should create an instance of SubStore and MsgStore, passing
// the Store's channel limits. Then, it should create a ChannelStore structure,
// which holds reference to those two stores, and return it, along with a boolean
// indicating if the channel has been created during this call. If the channel
// does exist, then LookupOrCreateChannel() behaves like LookupChannel() and
// the boolean returned is false.
//
// The LookupChannel() method should only return a ChannelStore that has been
// previously created, and nil if it does not exist.
//
type Store interface {
	// Init can be used to initialize the store with server's information.
	Init(info *spb.ServerInfo) error

	// Name returns the name type of this store (e.g: MEMORY, FILESTORE, etc...).
	Name() string

	// SetChannelLimits sets limits per channel. The action is not expected
	// to be retroactive.
	SetChannelLimits(limits ChannelLimits)

	// LookupOrCreateChannel returns a ChannelStore for the given channel,
	// or creates one if the channel doesn't exist. In this case, the returned
	// boolean will be true.
	LookupOrCreateChannel(channel string) (*ChannelStore, bool, error)

	// LookupChannel returns a ChannelStore for the given channel, nil if channel
	// does not exist.
	LookupChannel(channel string) *ChannelStore

	// HasChannel returns true if this store has any channel.
	HasChannel() bool

	// MsgsState returns message store statistics for a given channel, or all
	// if 'channel' is AllChannels.
	MsgsState(channel string) (numMessages int, byteSize uint64, err error)

	// AddClient stores information about the client identified by `clientID`.
	AddClient(clientID, hbInbox string) error

	// DeleteClient invalidates the client identified by `clientID`.
	DeleteClient(clientID string)

	// Close closes all stores.
	Close() error
}

// SubStore is the interface for storage of Subscriptions on a given channel.
//
// Implementations of this interface should not attempt to validate that
// a subscription is valid (that is, has not been deleted) when processing
// updates.
type SubStore interface {
	// CreateSub records a new subscription represented by SubState. On success,
	// it records the subscription's ID in SubState.ID. This ID is to be used
	// by the other SubStore methods.
	CreateSub(*spb.SubState) error

	// UpdateSub updates a given subscription represented by SubState.
	UpdateSub(*spb.SubState) error

	// DeleteSub invalidates the subscription 'subid'.
	DeleteSub(subid uint64)

	// AddSeqPending adds the given message 'seqno' to the subscription 'subid'.
	AddSeqPending(subid, seqno uint64) error

	// AckSeqPending records that the given message 'seqno' has been acknowledged
	// by the subscription 'subid'.
	AckSeqPending(subid, seqno uint64) error

	// Close closes the subscriptions store.
	Close() error
}

// MsgStore is the interface for storage of Messages on a given channel.
type MsgStore interface {
	// State returns some statistics related to this store.
	State() (numMessages int, byteSize uint64, err error)

	// Store stores a message.
	Store(reply string, data []byte) (*pb.MsgProto, error)

	// Lookup returns the stored message with given sequence number.
	Lookup(seq uint64) *pb.MsgProto

	// FirstSequence returns sequence for first message stored, 0 if no
	// message is stored.
	FirstSequence() uint64

	// LastSequence returns sequence for last message stored, 0 if no
	// message is stored.
	LastSequence() uint64

	// FirstAndLastSequence returns sequences for the first and last messages stored,
	// 0 if no message is stored.
	FirstAndLastSequence() (uint64, uint64)

	// GetSequenceFromTimestamp returns the sequence of the first message whose
	// timestamp is greater or equal to given timestamp.
	GetSequenceFromTimestamp(timestamp int64) uint64

	// FirstMsg returns the first message stored.
	FirstMsg() *pb.MsgProto

	// LastMsg returns the last message stored.
	LastMsg() *pb.MsgProto

	// Close closes the store.
	Close() error
}
