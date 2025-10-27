package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Announcement struct {
	ID               int64      `json:"id"`
	Title            string     `json:"title"`
	Message          string     `json:"message"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	IsActive         bool       `json:"is_active"`
	CreatedByAdminID int64      `json:"created_by_admin_id"`
}

type AnnouncementDelivery struct {
	ID                int64     `json:"id"`
	AnnouncementID    int64     `json:"announcement_id"`
	CustomerID        int64     `json:"customer_id"`
	TelegramMessageID *int      `json:"telegram_message_id"`
	DeliveredAt       time.Time `json:"delivered_at"`
}

type AnnouncementRepository struct {
	pool *pgxpool.Pool
}

func NewAnnouncementRepository(pool *pgxpool.Pool) *AnnouncementRepository {
	return &AnnouncementRepository{pool: pool}
}

func (r *AnnouncementRepository) Create(ctx context.Context, announcement *Announcement) (*Announcement, error) {
	query := `
		INSERT INTO announcement (title, message, expires_at, created_by_admin_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, is_active
	`

	err := r.pool.QueryRow(ctx, query, announcement.Title, announcement.Message, announcement.ExpiresAt, announcement.CreatedByAdminID).Scan(
		&announcement.ID,
		&announcement.CreatedAt,
		&announcement.IsActive,
	)

	return announcement, err
}

func (r *AnnouncementRepository) FindActiveAnnouncements(ctx context.Context) (*[]Announcement, error) {
	query := `
		SELECT id, title, message, created_at, expires_at, is_active, created_by_admin_id
		FROM announcement
		WHERE is_active = true AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var announcements []Announcement
	for rows.Next() {
		var a Announcement
		err := rows.Scan(&a.ID, &a.Title, &a.Message, &a.CreatedAt, &a.ExpiresAt, &a.IsActive, &a.CreatedByAdminID)
		if err != nil {
			return nil, err
		}
		announcements = append(announcements, a)
	}

	return &announcements, nil
}

func (r *AnnouncementRepository) DeactivateAnnouncement(ctx context.Context, id int64) error {
	query := `UPDATE announcement SET is_active = false WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *AnnouncementRepository) DeactivateExpiredAnnouncements(ctx context.Context) error {
	query := `UPDATE announcement SET is_active = false WHERE expires_at IS NOT NULL AND expires_at <= NOW() AND is_active = true`
	_, err := r.pool.Exec(ctx, query)
	return err
}

func (r *AnnouncementRepository) RecordDelivery(ctx context.Context, delivery *AnnouncementDelivery) error {
	query := `
		INSERT INTO announcement_delivery (announcement_id, customer_id, telegram_message_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (announcement_id, customer_id) DO UPDATE SET
			telegram_message_id = EXCLUDED.telegram_message_id,
			delivered_at = CURRENT_TIMESTAMP
	`
	_, err := r.pool.Exec(ctx, query, delivery.AnnouncementID, delivery.CustomerID, delivery.TelegramMessageID)
	return err
}

func (r *AnnouncementRepository) GetDeliveriesByAnnouncementID(ctx context.Context, announcementID int64) (*[]AnnouncementDelivery, error) {
	query := `
		SELECT id, announcement_id, customer_id, telegram_message_id, delivered_at
		FROM announcement_delivery
		WHERE announcement_id = $1
	`

	rows, err := r.pool.Query(ctx, query, announcementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []AnnouncementDelivery
	for rows.Next() {
		var d AnnouncementDelivery
		err := rows.Scan(&d.ID, &d.AnnouncementID, &d.CustomerID, &d.TelegramMessageID, &d.DeliveredAt)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}

	return &deliveries, nil
}

func (r *AnnouncementRepository) FindById(ctx context.Context, id int64) (*Announcement, error) {
	query := `
		SELECT id, title, message, created_at, expires_at, is_active, created_by_admin_id
		FROM announcement
		WHERE id = $1
	`

	var a Announcement
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.Title, &a.Message, &a.CreatedAt, &a.ExpiresAt, &a.IsActive, &a.CreatedByAdminID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to find announcement: %w", err)
	}

	return &a, nil
}
