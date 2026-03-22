package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"Graft/internal/crypto"
	"Graft/internal/models"
)

const destinationHeadersPrefix = "enc:v1:"

// Repository defines data access for rules and deliveries.
type Repository interface {
	SaveRule(ctx context.Context, rule models.Rule) error
	GetRuleByPath(ctx context.Context, path string) (*models.Rule, error)
	GetRuleByID(ctx context.Context, id string) (*models.Rule, error)
	ListRules(ctx context.Context) ([]models.Rule, error)
	DeleteRule(ctx context.Context, id string) error
	SaveDelivery(ctx context.Context, d models.Delivery) error
	ListDeliveriesByRule(ctx context.Context, ruleID string, limit int) ([]models.Delivery, error)
}

// SQLiteRepo implements Repository.
type SQLiteRepo struct {
	db        *sql.DB
	masterKey string
}

// NewSQLiteRepo creates a new SQLite repository and initializes schema.
func NewSQLiteRepo(db *sql.DB, masterKey string) (*SQLiteRepo, error) {
	schema := `
	CREATE TABLE IF NOT EXISTS rules (
		id TEXT PRIMARY KEY,
		name TEXT,
		description TEXT,
		listen_path TEXT UNIQUE,
		required_signature BOOLEAN,
		signature_header TEXT,
		signature_format TEXT,
		signature_timestamp_header TEXT,
		signature_max_skew_seconds INTEGER,
		signature_secret TEXT,
		transform_template TEXT,
		destination_url TEXT,
		destination_method TEXT,
		destination_headers TEXT
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init db schema: %w", err)
	}

	deliveries := `
	CREATE TABLE IF NOT EXISTS deliveries (
		id TEXT PRIMARY KEY,
		rule_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		success INTEGER NOT NULL,
		status_code INTEGER,
		error_message TEXT,
		duration_ms INTEGER NOT NULL,
		retry_count INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_deliveries_rule_id ON deliveries(rule_id, created_at DESC);
	`
	if _, err := db.Exec(deliveries); err != nil {
		return nil, fmt.Errorf("failed to init deliveries schema: %w", err)
	}

	repo := &SQLiteRepo{db: db, masterKey: masterKey}
	if err := repo.migrateLegacyColumns(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *SQLiteRepo) migrateLegacyColumns() error {
	// Best-effort ALTERs for DBs created before new columns existed.
	alters := []string{
		`ALTER TABLE rules ADD COLUMN signature_format TEXT DEFAULT 'hex';`,
		`ALTER TABLE rules ADD COLUMN signature_timestamp_header TEXT DEFAULT '';`,
		`ALTER TABLE rules ADD COLUMN signature_max_skew_seconds INTEGER DEFAULT 0;`,
	}
	for _, q := range alters {
		if _, err := r.db.Exec(q); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				return fmt.Errorf("migration: %w", err)
			}
		}
	}
	return nil
}

func (r *SQLiteRepo) encodeDestinationHeaders(h map[string]string) (string, error) {
	if len(h) == 0 {
		return "", nil
	}
	b, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	enc, err := crypto.Encrypt(string(b), r.masterKey)
	if err != nil {
		return "", err
	}
	return destinationHeadersPrefix + enc, nil
}

func (r *SQLiteRepo) decodeDestinationHeaders(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	if strings.HasPrefix(raw, destinationHeadersPrefix) {
		hexPart := strings.TrimPrefix(raw, destinationHeadersPrefix)
		plain, err := crypto.Decrypt(hexPart, r.masterKey)
		if err != nil {
			return nil, err
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(plain), &m); err != nil {
			return nil, err
		}
		if m == nil {
			return map[string]string{}, nil
		}
		return m, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("legacy destination_headers: %w", err)
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func (r *SQLiteRepo) SaveRule(ctx context.Context, rule models.Rule) error {
	headersStored, err := r.encodeDestinationHeaders(rule.DestinationHeaders)
	if err != nil {
		return fmt.Errorf("destination headers: %w", err)
	}
	if rule.SignatureFormat == "" {
		rule.SignatureFormat = "hex"
	}
	query := `
		INSERT INTO rules (
			id, name, description, listen_path, required_signature,
			signature_header, signature_format, signature_timestamp_header, signature_max_skew_seconds,
			signature_secret, transform_template,
			destination_url, destination_method, destination_headers
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			description=excluded.description,
			listen_path=excluded.listen_path,
			required_signature=excluded.required_signature,
			signature_header=excluded.signature_header,
			signature_format=excluded.signature_format,
			signature_timestamp_header=excluded.signature_timestamp_header,
			signature_max_skew_seconds=excluded.signature_max_skew_seconds,
			signature_secret=excluded.signature_secret,
			transform_template=excluded.transform_template,
			destination_url=excluded.destination_url,
			destination_method=excluded.destination_method,
			destination_headers=excluded.destination_headers;
	`
	_, err = r.db.ExecContext(ctx, query,
		rule.ID, rule.Name, rule.Description, rule.ListenPath, rule.RequiredSignature,
		rule.SignatureHeader, rule.SignatureFormat, rule.SignatureTimestampHeader, rule.SignatureMaxSkewSeconds,
		rule.SignatureSecret, rule.TransformTemplate,
		rule.DestinationURL, rule.DestinationMethod, headersStored,
	)
	return err
}

func (r *SQLiteRepo) scanRule(row *sql.Row) (*models.Rule, error) {
	var rule models.Rule
	var headersRaw string
	err := row.Scan(
		&rule.ID, &rule.Name, &rule.Description, &rule.ListenPath, &rule.RequiredSignature,
		&rule.SignatureHeader, &rule.SignatureFormat, &rule.SignatureTimestampHeader, &rule.SignatureMaxSkewSeconds,
		&rule.SignatureSecret, &rule.TransformTemplate,
		&rule.DestinationURL, &rule.DestinationMethod, &headersRaw,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if rule.SignatureFormat == "" {
		rule.SignatureFormat = "hex"
	}
	h, err := r.decodeDestinationHeaders(headersRaw)
	if err != nil {
		return nil, err
	}
	rule.DestinationHeaders = h
	return &rule, nil
}

const ruleSelect = `SELECT id, name, description, listen_path, required_signature,
	signature_header, signature_format, signature_timestamp_header, signature_max_skew_seconds,
	signature_secret, transform_template, destination_url, destination_method, destination_headers
	FROM rules`

func (r *SQLiteRepo) GetRuleByPath(ctx context.Context, path string) (*models.Rule, error) {
	q := ruleSelect + ` WHERE listen_path = ? LIMIT 1`
	return r.scanRule(r.db.QueryRowContext(ctx, q, path))
}

func (r *SQLiteRepo) GetRuleByID(ctx context.Context, id string) (*models.Rule, error) {
	q := ruleSelect + ` WHERE id = ? LIMIT 1`
	return r.scanRule(r.db.QueryRowContext(ctx, q, id))
}

func (r *SQLiteRepo) ListRules(ctx context.Context) ([]models.Rule, error) {
	rows, err := r.db.QueryContext(ctx, ruleSelect+` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Rule
	for rows.Next() {
		var rule models.Rule
		var headersRaw string
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.ListenPath, &rule.RequiredSignature,
			&rule.SignatureHeader, &rule.SignatureFormat, &rule.SignatureTimestampHeader, &rule.SignatureMaxSkewSeconds,
			&rule.SignatureSecret, &rule.TransformTemplate,
			&rule.DestinationURL, &rule.DestinationMethod, &headersRaw,
		); err != nil {
			return nil, err
		}
		if rule.SignatureFormat == "" {
			rule.SignatureFormat = "hex"
		}
		h, err := r.decodeDestinationHeaders(headersRaw)
		if err != nil {
			return nil, err
		}
		rule.DestinationHeaders = h
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *SQLiteRepo) DeleteRule(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepo) SaveDelivery(ctx context.Context, d models.Delivery) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deliveries (id, rule_id, created_at, success, status_code, error_message, duration_ms, retry_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.RuleID, d.CreatedAt, boolToInt(d.Success), d.StatusCode, d.ErrorMsg, d.DurationMS, d.RetryCount,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *SQLiteRepo) ListDeliveriesByRule(ctx context.Context, ruleID string, limit int) ([]models.Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, rule_id, created_at, success, status_code, error_message, duration_ms, retry_count
		FROM deliveries WHERE rule_id = ? ORDER BY created_at DESC LIMIT ?`,
		ruleID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Delivery
	for rows.Next() {
		var d models.Delivery
		var successInt int
		if err := rows.Scan(&d.ID, &d.RuleID, &d.CreatedAt, &successInt, &d.StatusCode, &d.ErrorMsg, &d.DurationMS, &d.RetryCount); err != nil {
			return nil, err
		}
		d.Success = successInt != 0
		out = append(out, d)
	}
	return out, rows.Err()
}
