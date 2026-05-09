package storage

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database struct {
	db *gorm.DB
}

type SecretRequest struct {
	ID                string     `gorm:"primaryKey" json:"request_id"`
	ClientID          string     `gorm:"index" json:"client_id"`
	SecretPath        string     `gorm:"index" json:"secret_path"`
	Reason            string     `json:"reason"`
	Status            string     `gorm:"index" json:"status"`
	StatusDetail      string     `json:"status_detail"`
	CreatedAt         time.Time  `json:"created_at"`
	ExpiresAt         time.Time  `json:"expires_at"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty"`
	DeniedAt          *time.Time `json:"denied_at,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	RequiredApprovals int        `json:"required_approvals"`
	ApprovalCount     int        `json:"approval_count"`
	DenialCount       int        `json:"denial_count"`
	Token             string     `json:"token,omitempty"`
	TokenExpiresAt    *time.Time `json:"token_expires_at,omitempty"`
}

type Approval struct {
	ID         string    `gorm:"primaryKey"`
	RequestID  string    `gorm:"index"`
	Identity   string    `json:"identity"`
	ApprovedAt time.Time `json:"approved_at"`
	Reason     string    `json:"reason"`
}

type Denial struct {
	ID        string    `gorm:"primaryKey"`
	RequestID string    `gorm:"index"`
	Identity  string    `json:"identity"`
	DeniedAt  time.Time `json:"denied_at"`
	Reason    string    `json:"reason"`
}

type ListFilters struct {
	Status     string
	ClientID   string
	SecretPath string
	From       *time.Time
	To         *time.Time
}

func New(storageType, connectionString string) (*Database, error) {
	var db *gorm.DB
	var err error

	switch storageType {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
	case "postgres":
		db, err = gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate schemas
	if err := db.AutoMigrate(&SecretRequest{}, &Approval{}, &Denial{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) CreateRequest(req *SecretRequest) error {
	return d.db.Create(req).Error
}

func (d *Database) GetRequest(id string) (*SecretRequest, error) {
	var req SecretRequest
	if err := d.db.First(&req, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("request not found: %s", id)
		}
		return nil, err
	}
	return &req, nil
}

func (d *Database) UpdateRequestStatus(id string, status string) error {
	return d.db.Model(&SecretRequest{}).Where("id = ?", id).Update("status", status).Error
}

func (d *Database) ListRequests(filters ListFilters, limit int, offset int) ([]*SecretRequest, int, error) {
	var requests []*SecretRequest
	var total int64

	query := d.db.Model(&SecretRequest{})

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.ClientID != "" {
		query = query.Where("client_id = ?", filters.ClientID)
	}
	if filters.SecretPath != "" {
		query = query.Where("secret_path LIKE ?", filters.SecretPath+"%")
	}
	if filters.From != nil {
		query = query.Where("created_at >= ?", *filters.From)
	}
	if filters.To != nil {
		query = query.Where("created_at <= ?", *filters.To)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Limit(limit).Offset(offset).Find(&requests).Error; err != nil {
		return nil, 0, err
	}

	return requests, int(total), nil
}

func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
