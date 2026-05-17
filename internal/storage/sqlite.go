package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/DongDuong2001/graft/internal/crypto"
	"github.com/DongDuong2001/graft/internal/models"
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
	GetDeliveryByID(ctx context.Context, id string) (*models.Delivery, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status string, err string) error
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
		status TEXT NOT NULL DEFAULT 'pending',
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
	// --- Best-effort ALTERs for DBs created before new columns existed ---
	alters := []string{
		`ALTER TABLE rules ADD COLUMN signature_format TEXT DEFAULT 'hex';`,
		`ALTER TABLE rules ADD COLUMN signature_timestamp_header TEXT DEFAULT '';`,
		`ALTER TABLE rules ADD COLUMN signature_max_skew_seconds INTEGER DEFAULT 0;`,
		// --- Fan-out: JSON array of destination objects ---
		`ALTER TABLE rules ADD COLUMN destinations TEXT DEFAULT '';`,
		// --- Conditional routing: JSON array of condition rules ---
		`ALTER TABLE rules ADD COLUMN conditions TEXT DEFAULT '';`,
		// --- Pipeline: JSON array of transformation steps ---
		`ALTER TABLE rules ADD COLUMN transform_steps TEXT DEFAULT '';`,
		// --- Per-rule rate limiting ---
		`ALTER TABLE rules ADD COLUMN rate_limit_max INTEGER DEFAULT 0;`,
		`ALTER TABLE rules ADD COLUMN rate_limit_window TEXT DEFAULT '';`,
		// --- Per-rule IP allowlist (JSON array of CIDR strings) ---
		`ALTER TABLE rules ADD COLUMN ip_allowlist TEXT DEFAULT '';`,
		// --- Delivery status column for existing databases ---
		`ALTER TABLE deliveries ADD COLUMN status TEXT NOT NULL DEFAULT 'pending';`,
		// --- Batch 2: Delivery replay payload capture ---
		`ALTER TABLE deliveries ADD COLUMN request_body BLOB;`,
		`ALTER TABLE deliveries ADD COLUMN request_path TEXT DEFAULT '';`,
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

// --- SaveRule persists a rule with all legacy and new fields ---
func (r *SQLiteRepo) SaveRule(ctx context.Context, rule models.Rule) error {
	headersStored, err := r.encodeDestinationHeaders(rule.DestinationHeaders)
	if err != nil {
		return fmt.Errorf("destination headers: %w", err)
	}
	if rule.SignatureFormat == "" {
		rule.SignatureFormat = "hex"
	}

	// --- Serialize new JSON columns ---
	destJSON := encodeJSON(rule.Destinations)
	condJSON := encodeJSON(rule.Conditions)
	stepsJSON := encodeJSON(rule.TransformSteps)
	allowlistJSON := encodeJSON(rule.IPAllowlist)

	query := `
		INSERT INTO rules (
			id, name, description, listen_path, required_signature,
			signature_header, signature_format, signature_timestamp_header, signature_max_skew_seconds,
			signature_secret, transform_template,
			destination_url, destination_method, destination_headers,
			destinations, conditions, transform_steps,
			rate_limit_max, rate_limit_window, ip_allowlist
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			destination_headers=excluded.destination_headers,
			destinations=excluded.destinations,
			conditions=excluded.conditions,
			transform_steps=excluded.transform_steps,
			rate_limit_max=excluded.rate_limit_max,
			rate_limit_window=excluded.rate_limit_window,
			ip_allowlist=excluded.ip_allowlist;
	`
	_, err = r.db.ExecContext(ctx, query,
		rule.ID, rule.Name, rule.Description, rule.ListenPath, rule.RequiredSignature,
		rule.SignatureHeader, rule.SignatureFormat, rule.SignatureTimestampHeader, rule.SignatureMaxSkewSeconds,
		rule.SignatureSecret, rule.TransformTemplate,
		rule.DestinationURL, rule.DestinationMethod, headersStored,
		destJSON, condJSON, stepsJSON,
		rule.RateLimitMax, rule.RateLimitWindow, allowlistJSON,
	)
	return err
}

// --- encodeJSON serializes any value to a JSON string; returns "" on nil/empty ---
func encodeJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(b)
	if s == "null" || s == "[]" {
		return ""
	}
	return s
}

// --- scanRule decodes a single rule row including new JSON columns ---
func (r *SQLiteRepo) scanRule(row *sql.Row) (*models.Rule, error) {
	var rule models.Rule
	var headersRaw, destRaw, condRaw, stepsRaw, allowlistRaw string
	err := row.Scan(
		&rule.ID, &rule.Name, &rule.Description, &rule.ListenPath, &rule.RequiredSignature,
		&rule.SignatureHeader, &rule.SignatureFormat, &rule.SignatureTimestampHeader, &rule.SignatureMaxSkewSeconds,
		&rule.SignatureSecret, &rule.TransformTemplate,
		&rule.DestinationURL, &rule.DestinationMethod, &headersRaw,
		&destRaw, &condRaw, &stepsRaw,
		&rule.RateLimitMax, &rule.RateLimitWindow, &allowlistRaw,
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

	// --- Decode new JSON fields ---
	decodeJSON(destRaw, &rule.Destinations)
	decodeJSON(condRaw, &rule.Conditions)
	decodeJSON(stepsRaw, &rule.TransformSteps)
	decodeJSON(allowlistRaw, &rule.IPAllowlist)

	return &rule, nil
}

// --- decodeJSON unmarshals a JSON string into the target; ignores empty/null ---
func decodeJSON(raw string, target interface{}) {
	if raw == "" || raw == "null" || raw == "[]" {
		return
	}
	_ = json.Unmarshal([]byte(raw), target)
}

// --- ruleSelect lists all columns including new ones ---
const ruleSelect = `SELECT id, name, description, listen_path, required_signature,
	signature_header, signature_format, signature_timestamp_header, signature_max_skew_seconds,
	signature_secret, transform_template, destination_url, destination_method, destination_headers,
	destinations, conditions, transform_steps,
	rate_limit_max, rate_limit_window, ip_allowlist
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

	// --- Scan all rows including new fan-out and routing columns ---
	var out []models.Rule
	for rows.Next() {
		var rule models.Rule
		var headersRaw, destRaw, condRaw, stepsRaw, allowlistRaw string
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Description, &rule.ListenPath, &rule.RequiredSignature,
			&rule.SignatureHeader, &rule.SignatureFormat, &rule.SignatureTimestampHeader, &rule.SignatureMaxSkewSeconds,
			&rule.SignatureSecret, &rule.TransformTemplate,
			&rule.DestinationURL, &rule.DestinationMethod, &headersRaw,
			&destRaw, &condRaw, &stepsRaw,
			&rule.RateLimitMax, &rule.RateLimitWindow, &allowlistRaw,
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
		decodeJSON(destRaw, &rule.Destinations)
		decodeJSON(condRaw, &rule.Conditions)
		decodeJSON(stepsRaw, &rule.TransformSteps)
		decodeJSON(allowlistRaw, &rule.IPAllowlist)
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
	if d.Status == "" {
		d.Status = models.StatusPending
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deliveries (
			id, rule_id, created_at, success, status, status_code, error_message, duration_ms, retry_count,
			request_body, request_path
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			success=excluded.success,
			status=excluded.status,
			status_code=excluded.status_code,
			error_message=excluded.error_message,
			duration_ms=excluded.duration_ms,
			retry_count=excluded.retry_count,
			request_body=excluded.request_body,
			request_path=excluded.request_path`,
		d.ID, d.RuleID, d.CreatedAt, boolToInt(d.Success), d.Status, d.StatusCode, d.ErrorMsg, d.DurationMS, d.RetryCount,
		d.RequestBody, d.RequestPath,
	)
	return err
}

func (r *SQLiteRepo) UpdateDeliveryStatus(ctx context.Context, id string, status string, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE deliveries SET status = ?, error_message = ? WHERE id = ?`, status, errMsg, id)
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
		SELECT id, rule_id, created_at, success, status, status_code, error_message, duration_ms, retry_count, request_body, request_path
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
		if err := rows.Scan(&d.ID, &d.RuleID, &d.CreatedAt, &successInt, &d.Status, &d.StatusCode, &d.ErrorMsg, &d.DurationMS, &d.RetryCount, &d.RequestBody, &d.RequestPath); err != nil {
			return nil, err
		}
		d.Success = successInt == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDeliveryByID fetches a specific delivery including its captured request body.
func (r *SQLiteRepo) GetDeliveryByID(ctx context.Context, id string) (*models.Delivery, error) {
	var d models.Delivery
	var successInt int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, rule_id, created_at, success, status, status_code, error_message, duration_ms, retry_count, request_body, request_path
		FROM deliveries WHERE id = ? LIMIT 1`,
		id,
	).Scan(&d.ID, &d.RuleID, &d.CreatedAt, &successInt, &d.Status, &d.StatusCode, &d.ErrorMsg, &d.DurationMS, &d.RetryCount, &d.RequestBody, &d.RequestPath)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil for not found instead of error for service matching
		}
		return nil, err
	}
	d.Success = successInt == 1
	return &d, nil
}
