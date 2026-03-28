package utils

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

// TelegramMediaMetadata describes the media attached to a Telegram message.
type TelegramMediaMetadata struct {
	HasMedia  bool
	MediaType string
	FileID    string
	FileName  string
	MimeType  string
}

// ExtractTelegramMediaMetadata returns normalized media metadata for a message.
func ExtractTelegramMediaMetadata(message *models.Message) TelegramMediaMetadata {
	if message == nil {
		return TelegramMediaMetadata{}
	}

	switch {
	case len(message.Photo) > 0:
		photo := message.Photo[len(message.Photo)-1]
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "photo",
			FileID:    strings.TrimSpace(photo.FileID),
			MimeType:  "image/jpeg",
		}
	case message.Document != nil:
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "document",
			FileID:    strings.TrimSpace(message.Document.FileID),
			FileName:  strings.TrimSpace(message.Document.FileName),
			MimeType:  strings.TrimSpace(message.Document.MimeType),
		}
	case message.Video != nil:
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "video",
			FileID:    strings.TrimSpace(message.Video.FileID),
			FileName:  strings.TrimSpace(message.Video.FileName),
			MimeType:  strings.TrimSpace(message.Video.MimeType),
		}
	case message.Voice != nil:
		mimeType := strings.TrimSpace(message.Voice.MimeType)
		if mimeType == "" {
			mimeType = "audio/ogg"
		}
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "voice",
			FileID:    strings.TrimSpace(message.Voice.FileID),
			MimeType:  mimeType,
		}
	case message.Audio != nil:
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "audio",
			FileID:    strings.TrimSpace(message.Audio.FileID),
			FileName:  strings.TrimSpace(message.Audio.FileName),
			MimeType:  strings.TrimSpace(message.Audio.MimeType),
		}
	case message.Sticker != nil:
		mimeType := "image/webp"
		switch {
		case message.Sticker.IsAnimated:
			mimeType = "application/x-tgsticker"
		case message.Sticker.IsVideo:
			mimeType = "video/webm"
		}
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "sticker",
			FileID:    strings.TrimSpace(message.Sticker.FileID),
			MimeType:  mimeType,
		}
	case message.VideoNote != nil:
		return TelegramMediaMetadata{
			HasMedia:  true,
			MediaType: "video_note",
			FileID:    strings.TrimSpace(message.VideoNote.FileID),
			MimeType:  "video/mp4",
		}
	default:
		return TelegramMediaMetadata{}
	}
}
