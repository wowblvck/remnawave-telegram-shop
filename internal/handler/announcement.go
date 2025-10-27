package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (h *Handler) CreateAnnouncementCommandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	langCode := update.Message.From.LanguageCode

	text := strings.TrimSpace(update.Message.Text)
	parts := strings.SplitN(text, " ", 2)

	if len(parts) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      h.translation.GetText(langCode, "announce_command_help"),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	args := parts[1]
	title, message, hours, err := h.parseAnnouncementArgs(args, langCode)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf(h.translation.GetText(langCode, "announce_parse_error"), err.Error()),
		})
		return
	}

	var expiresAt *time.Time
	if hours > 0 {
		expiry := time.Now().Add(time.Duration(hours) * time.Hour)
		expiresAt = &expiry
	}

	announcement, err := h.announcementService.CreateAnnouncement(ctx, title, message, expiresAt, update.Message.From.ID)
	if err != nil {
		slog.Error("Failed to create announcement", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_create_error"),
		})
		return
	}

	var expiryText string
	if expiresAt != nil {
		expiryText = fmt.Sprintf(h.translation.GetText(langCode, "announce_expiry_time"), hours)
	} else {
		expiryText = h.translation.GetText(langCode, "announce_permanent")
	}

	_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: fmt.Sprintf(h.translation.GetText(langCode, "announce_created_message"),
			announcement.Title,
			announcement.Message,
			expiryText,
			announcement.ID),
		ParseMode: models.ParseModeHTML,
	})
	if sendErr != nil {
		slog.Error("failed to send confirmation to creator", "error", sendErr)
	}
}

func (h *Handler) DeleteAnnouncementCommandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	langCode := update.Message.From.LanguageCode

	text := strings.TrimSpace(update.Message.Text)
	parts := strings.Fields(text)

	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_delete_help"),
		})
		return
	}

	announcementID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_invalid_id"),
		})
		return
	}

	err = h.announcementService.DeleteAnnouncement(ctx, announcementID)
	if err != nil {
		slog.Error("Failed to delete announcement", "announcementID", announcementID, "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_delete_error"),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf(h.translation.GetText(langCode, "announce_deleted"), announcementID),
	})
}

func (h *Handler) ListAnnouncementsCommandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	langCode := update.Message.From.LanguageCode

	announcements, err := h.announcementService.GetActiveAnnouncements(ctx)
	if err != nil {
		slog.Error("Failed to get announcements", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_list_error"),
		})
		return
	}

	if len(*announcements) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_no_active"),
		})
		return
	}

	var response strings.Builder
	response.WriteString(h.translation.GetText(langCode, "announce_active_list"))

	for _, announcement := range *announcements {
		response.WriteString(fmt.Sprintf(h.translation.GetText(langCode, "announce_list_message"), announcement.ID, announcement.Title, announcement.Message, announcement.CreatedAt.Format("02.01.2006 15:04")))

		if announcement.ExpiresAt != nil {
			response.WriteString(fmt.Sprintf(h.translation.GetText(langCode, "announce_expires_at"), announcement.ExpiresAt.Format("02.01.2006 15:04")))
		} else {
			response.WriteString(h.translation.GetText(langCode, "announce_permanent_status"))
		}
		response.WriteString("\n---\n\n")
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      response.String(),
		ParseMode: models.ParseModeHTML,
	})
}

func (h *Handler) DeleteAllAnnouncementsCommandHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	langCode := update.Message.From.LanguageCode

	text := strings.TrimSpace(update.Message.Text)
	parts := strings.Fields(text)
	slog.Any("parts", parts)
	if len(parts) > 1 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_delete_all_help"),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   h.translation.GetText(langCode, "announce_delete_all_start"),
	})

	totalDeleted, totalFailed, err := h.announcementService.DeleteAllAnnouncements(ctx)
	if err != nil {
		slog.Error("Failed to delete all announcements", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   h.translation.GetText(langCode, "announce_delete_error"),
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf(h.translation.GetText(langCode, "announce_delete_all_complete"), totalDeleted, totalFailed),
	})
}

func (h *Handler) parseAnnouncementArgs(args string, langCode string) (title, message string, hours int, err error) {
	var parts []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range args {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if r == '"' {
			if inQuotes {
				parts = append(parts, current.String())
				current.Reset()
				inQuotes = false
			} else {
				inQuotes = true
			}
			continue
		}

		if inQuotes {
			current.WriteRune(r)
		} else if r == ' ' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if len(parts) < 2 {
		return "", "", 0, fmt.Errorf("%s", h.translation.GetText(langCode, "announce_parse_error_missing_quotes"))
	}

	title = parts[0]
	message = parts[1]

	if strings.TrimSpace(title) == "" {
		return "", "", 0, fmt.Errorf("%s", h.translation.GetText(langCode, "announce_parse_error_empty_title"))
	}

	if strings.TrimSpace(message) == "" {
		return "", "", 0, fmt.Errorf("%s", h.translation.GetText(langCode, "announce_parse_error_empty_message"))
	}

	if len(parts) > 2 {
		hour, parseErr := strconv.Atoi(parts[2])
		if parseErr != nil {
			return "", "", 0, fmt.Errorf(h.translation.GetText(langCode, "announce_parse_error_invalid_hours"), parts[2])
		}
		hours = hour
	}

	return title, message, hours, nil
}
