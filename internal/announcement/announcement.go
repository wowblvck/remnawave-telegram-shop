package announcement

import (
	"context"
	"fmt"
	"log/slog"
	"remnawave-tg-shop-bot/internal/database"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type AnnouncementService struct {
	announcementRepo *database.AnnouncementRepository
	customerRepo     *database.CustomerRepository
	bot              *bot.Bot
}

func NewAnnouncementService(announcementRepo *database.AnnouncementRepository, customerRepo *database.CustomerRepository, bot *bot.Bot) *AnnouncementService {
	return &AnnouncementService{
		announcementRepo: announcementRepo,
		customerRepo:     customerRepo,
		bot:              bot,
	}
}

func (s *AnnouncementService) CreateAnnouncement(ctx context.Context, title, message string, expiresAt *time.Time, adminID int64) (*database.Announcement, error) {
	announcement := &database.Announcement{
		Title:            title,
		Message:          message,
		ExpiresAt:        expiresAt,
		CreatedByAdminID: adminID,
	}

	createdAnnouncement, err := s.announcementRepo.Create(ctx, announcement)
	if err != nil {
		return nil, err
	}

	go s.sendAnnouncementToAllUsers(context.Background(), createdAnnouncement)

	return createdAnnouncement, nil
}

func (s *AnnouncementService) sendAnnouncementToAllUsers(ctx context.Context, announcement *database.Announcement) {
	customers, err := s.customerRepo.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to get customers for announcement", "error", err)
		return
	}

	successCount := 0
	failCount := 0

	for _, customer := range *customers {
		messageText := fmt.Sprintf("📢 <b>%s</b>\n\n%s", announcement.Title, announcement.Message)

		msg, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    customer.TelegramID,
			Text:      messageText,
			ParseMode: models.ParseModeHTML,
		})

		if err != nil {
			slog.Error("Failed to send announcement to user", "customerID", customer.ID, "telegramID", customer.TelegramID, "error", err)
			failCount++
			continue
		}

		delivery := &database.AnnouncementDelivery{
			AnnouncementID:    announcement.ID,
			CustomerID:        customer.ID,
			TelegramMessageID: &msg.ID,
		}

		err = s.announcementRepo.RecordDelivery(ctx, delivery)
		if err != nil {
			slog.Error("Failed to record announcement delivery", "error", err)
		}

		successCount++
	}

	slog.Info("Announcement delivery completed",
		"announcementID", announcement.ID,
		"success", successCount,
		"failed", failCount)
}

func (s *AnnouncementService) DeleteAnnouncement(ctx context.Context, announcementID int64) error {
	deliveries, err := s.announcementRepo.GetDeliveriesByAnnouncementID(ctx, announcementID)
	if err != nil {
		return err
	}

	deletedCount := 0
	failedCount := 0

	for _, delivery := range *deliveries {
		if delivery.TelegramMessageID != nil {
			customer, err := s.customerRepo.FindById(ctx, delivery.CustomerID)
			if err != nil {
				failedCount++
				continue
			}

			_, err = s.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    customer.TelegramID,
				MessageID: int(*delivery.TelegramMessageID),
			})
			if err != nil {
				slog.Error("Failed to delete announcement message", "customerID", customer.ID, "messageID", *delivery.TelegramMessageID, "error", err)
				failedCount++
			} else {
				deletedCount++
			}
		}
	}

	slog.Info("Announcement messages deletion completed",
		"announcementID", announcementID,
		"deleted", deletedCount,
		"failed", failedCount)

	return s.announcementRepo.DeactivateAnnouncement(ctx, announcementID)
}

func (s *AnnouncementService) SendActiveAnnouncementsToNewUser(ctx context.Context, userID int64, customerID int64) {
	announcements, err := s.announcementRepo.FindActiveAnnouncements(ctx)
	if err != nil {
		slog.Error("Failed to get active announcements for new user", "error", err)
		return
	}

	successCount := 0
	failCount := 0

	for _, announcement := range *announcements {
		messageText := fmt.Sprintf("📢 <b>%s</b>\n\n%s", announcement.Title, announcement.Message)

		msg, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messageText,
			ParseMode: models.ParseModeHTML,
		})

		if err != nil {
			slog.Error("Failed to send announcement to new user", "customerID", customerID, "telegramID", userID, "error", err)
			failCount++
			continue
		}

		delivery := &database.AnnouncementDelivery{
			AnnouncementID:    announcement.ID,
			CustomerID:        customerID,
			TelegramMessageID: &msg.ID,
		}

		err = s.announcementRepo.RecordDelivery(ctx, delivery)
		if err != nil {
			slog.Error("Failed to record announcement delivery", "error", err)
		}

		successCount++
		time.Sleep(500 * time.Millisecond) // Небольшая задержка между сообщениями
	}

	slog.Info("Announcements sent to new user",
		"customerID", customerID,
		"success", successCount,
		"failed", failCount)
}


func (s *AnnouncementService) GetActiveAnnouncements(ctx context.Context) (*[]database.Announcement, error) {
	return s.announcementRepo.FindActiveAnnouncements(ctx)
}

func (s *AnnouncementService) CleanupExpiredAnnouncements(ctx context.Context) error {
	expiredIDs, err := s.announcementRepo.DeactivateExpiredAnnouncements(ctx)
	if err != nil {
		return err
	}

	if len(expiredIDs) == 0 {
		return nil
	}

	go func() {
		cleanupCtx := context.Background()

		for _, announcementID := range expiredIDs {
			deliveries, err := s.announcementRepo.GetDeliveriesByAnnouncementID(cleanupCtx, announcementID)
			if err != nil {
				slog.Error("Failed to get deliveries for expired announcement", "announcementID", announcementID, "error", err)
				continue
			}

			deletedCount := 0
			failedCount := 0

			for _, delivery := range *deliveries {
				if delivery.TelegramMessageID != nil {
					customer, err := s.customerRepo.FindById(cleanupCtx, delivery.CustomerID)
					if err != nil {
						failedCount++
						continue
					}

					_, err = s.bot.DeleteMessage(cleanupCtx, &bot.DeleteMessageParams{
						ChatID:    customer.TelegramID,
						MessageID: int(*delivery.TelegramMessageID),
					})
					if err != nil {
						slog.Error("Failed to delete expired announcement message", "customerID", customer.ID, "messageID", *delivery.TelegramMessageID, "error", err)
						failedCount++
					} else {
						deletedCount++
					}
				}
			}

			slog.Info("Expired announcement messages deletion completed",
				"announcementID", announcementID,
				"deleted", deletedCount,
				"failed", failedCount)
		}
	}()
	return nil
}

func (s *AnnouncementService) DeleteAllAnnouncements(ctx context.Context) (int, int, error) {
	announcements, err := s.announcementRepo.FindActiveAnnouncements(ctx)
	if err != nil {
		return 0, 0, err
	}

	totalDeleted := 0
	totalFailed := 0

	for _, announcement := range *announcements {
		deliveries, err := s.announcementRepo.GetDeliveriesByAnnouncementID(ctx, announcement.ID)
		if err != nil {
			slog.Error("Failed to get deliveries for announcement", "announcementID", announcement.ID, "error", err)
			totalFailed++
			continue
		}

		deletedCount := 0
		failedCount := 0

		for _, delivery := range *deliveries {
			if delivery.TelegramMessageID != nil {
				customer, err := s.customerRepo.FindById(ctx, delivery.CustomerID)
				if err != nil {
					failedCount++
					continue
				}

				_, err = s.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
					ChatID:    customer.TelegramID,
					MessageID: int(*delivery.TelegramMessageID),
				})
				if err != nil {
					slog.Error("Failed to delete announcement message", "customerID", customer.ID, "messageID", *delivery.TelegramMessageID, "error", err)
					failedCount++
				} else {
					deletedCount++
				}
			}
		}

		slog.Info("Announcement messages deletion completed",
			"announcementID", announcement.ID,
			"deleted", deletedCount,
			"failed", failedCount)

		err = s.announcementRepo.DeactivateAnnouncement(ctx, announcement.ID)
		if err != nil {
			slog.Error("Failed to deactivate announcement", "announcementID", announcement.ID, "error", err)
			totalFailed++
		} else {
			totalDeleted++
		}
	}

	return totalDeleted, totalFailed, nil
}
