package defaultstore

import (
	"sync"

	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/internal/moreatomic"
	"github.com/diamondburned/arikawa/v2/state/store"
)

type Message struct {
	channels moreatomic.Map
	maxMsgs  int
}

var _ store.MessageStore = (*Message)(nil)

type messages struct {
	mut      sync.Mutex
	messages []discord.Message
}

func NewMessage(maxMsgs int) *Message {
	return &Message{
		channels: *moreatomic.NewMap(func() interface{} {
			return &messages{
				messages: []discord.Message{}, // never use a nil slice
			}
		}),
		maxMsgs: maxMsgs,
	}
}

func (s *Message) Reset() error {
	return s.channels.Reset()
}

func (s *Message) Message(chID discord.ChannelID, mID discord.MessageID) (*discord.Message, error) {
	iv, ok := s.channels.Load(chID)
	if !ok {
		return nil, store.ErrNotFound
	}

	msgs := iv.(*messages)

	msgs.mut.Lock()
	defer msgs.mut.Unlock()

	for _, m := range msgs.messages {
		if m.ID == mID {
			return &m, nil
		}
	}

	return nil, store.ErrNotFound
}

func (s *Message) Messages(channelID discord.ChannelID) ([]discord.Message, error) {
	iv, ok := s.channels.Load(channelID)
	if !ok {
		return nil, store.ErrNotFound
	}

	msgs := iv.(*messages)

	msgs.mut.Lock()
	defer msgs.mut.Unlock()

	return append([]discord.Message(nil), msgs.messages...), nil
}

func (s *Message) MaxMessages() int {
	return s.maxMsgs
}

func (s *Message) MessageSet(message discord.Message) error {
	iv, _ := s.channels.LoadOrStore(message.ChannelID)

	msgs := iv.(*messages)

	msgs.mut.Lock()
	defer msgs.mut.Unlock()

	// Check if we already have the message.
	for i, m := range msgs.messages {
		if m.ID == message.ID {
			DiffMessage(message, &m)
			msgs.messages[i] = m
			return nil
		}
	}

	// Order: latest to earliest, similar to the API.

	var end = len(msgs.messages)
	if max := s.MaxMessages(); end >= max {
		// If the end (length) is larger than the maximum amount, then cap it.
		end = max
	} else {
		// Else, append an empty message to the end.
		msgs.messages = append(msgs.messages, discord.Message{})
		// Increment to update the length.
		end++
	}

	// Copy hack to prepend. This copies the 0th-(end-1)th entries to
	// 1st-endth.
	copy(msgs.messages[1:end], msgs.messages[0:end-1])
	// Then, set the 0th entry.
	msgs.messages[0] = message

	return nil
}

// DiffMessage fills non-empty fields from src to dst.
func DiffMessage(src discord.Message, dst *discord.Message) {
	// Thanks, Discord.
	if src.Content != "" {
		dst.Content = src.Content
	}
	if src.EditedTimestamp.IsValid() {
		dst.EditedTimestamp = src.EditedTimestamp
	}
	if src.Mentions != nil {
		dst.Mentions = src.Mentions
	}
	if src.Embeds != nil {
		dst.Embeds = src.Embeds
	}
	if src.Attachments != nil {
		dst.Attachments = src.Attachments
	}
	if src.Timestamp.IsValid() {
		dst.Timestamp = src.Timestamp
	}
	if src.Author.ID.IsValid() {
		dst.Author = src.Author
	}
	if src.Reactions != nil {
		dst.Reactions = src.Reactions
	}
}

func (s *Message) MessageRemove(channelID discord.ChannelID, messageID discord.MessageID) error {
	iv, ok := s.channels.Load(channelID)
	if !ok {
		return nil
	}

	msgs := iv.(*messages)

	msgs.mut.Lock()
	defer msgs.mut.Unlock()

	for i, m := range msgs.messages {
		if m.ID == messageID {
			msgs.messages = append(msgs.messages[:i], msgs.messages[i+1:]...)
			return nil
		}
	}

	return nil
}
