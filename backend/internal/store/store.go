package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"genco/finance/internal/db"
	"genco/finance/internal/model"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu    sync.Mutex
	state model.State
	path  string
	db    *sql.DB
}

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Open(ctx context.Context, dir, dsn string) (*Store, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	s := &Store{state: model.NewState(), path: filepath.Join(dir, "state.json")}
	if dsn != "" {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(8)
		db.SetConnMaxLifetime(time.Minute * 5)
		if err = db.PingContext(ctx); err != nil {
			return nil, err
		}
		s.db = db
		payload, err := dbQueries(db).GetState(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			if err = s.persist(ctx, s.state); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, fmt.Errorf("database is not migrated: %w", err)
		} else if err = json.Unmarshal(payload, &s.state); err != nil {
			return nil, err
		}
	} else {
		b, err := os.ReadFile(s.path)
		if err == nil {
			if err = json.Unmarshal(b, &s.state); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
func (s *Store) Ready(ctx context.Context) error {
	if s.db != nil {
		return s.db.PingContext(ctx)
	}
	_, err := os.Stat(filepath.Dir(s.path))
	return err
}
func clone(v model.State) model.State {
	b, _ := json.Marshal(v)
	var out model.State
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) Read() model.State { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.state) }
func (s *Store) Change(ctx context.Context, fn func(*model.State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	if err := fn(&next); err != nil {
		return err
	}
	if err := s.persist(ctx, next); err != nil {
		return err
	}
	s.state = next
	return nil
}
func (s *Store) persist(ctx context.Context, st model.State) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if s.db == nil {
		tmp := s.path + ".tmp"
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if err = os.Rename(tmp, s.path); err != nil {
			return err
		}
		dir, err := os.Open(filepath.Dir(s.path))
		if err != nil {
			return err
		}
		defer dir.Close()
		return dir.Sync()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = dbQueries(tx).SetState(ctx, b); err != nil {
		return err
	}
	for _, setting := range st.Settings {
		v, _ := json.Marshal(setting)
		if _, err = tx.ExecContext(ctx, "INSERT IGNORE INTO preset_versions(version,payload,created_at) VALUES(?,?,?)", setting.Version, v, setting.UpdatedAt); err != nil {
			return err
		}
	}
	for _, p := range st.Products {
		var sku any
		if p.Input.SKU != "" {
			sku = p.Input.SKU
		}
		payload, _ := json.Marshal(p)
		_, err = tx.ExecContext(ctx, `INSERT INTO products(id,sku,name,category,cost,price,monthly_orders,fixed_costs,packaging,archived,edit_version,payload,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE sku=VALUES(sku),name=VALUES(name),category=VALUES(category),cost=VALUES(cost),price=VALUES(price),monthly_orders=VALUES(monthly_orders),fixed_costs=VALUES(fixed_costs),packaging=VALUES(packaging),archived=VALUES(archived),edit_version=VALUES(edit_version),payload=VALUES(payload),updated_at=VALUES(updated_at)`, p.ID, sku, p.Input.Name, p.Input.Category, p.Input.Cost, p.Input.Price, p.Input.Orders, p.Input.Fixed, p.Input.Packaging, p.Archived, p.Version, payload, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			return err
		}
		for _, ev := range p.Revisions {
			v, _ := json.Marshal(ev)
			_, err = tx.ExecContext(ctx, "INSERT IGNORE INTO evaluation_revisions(id,product_id,preset_version,calculation_version,payload,created_at) VALUES(?,?,?,?,?,?)", ev.ID, p.ID, ev.PresetVersion, ev.CalculationVersion, v, ev.CreatedAt)
			if err != nil {
				return err
			}
		}
		for _, table := range []string{"competitors", "product_images"} {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE product_id=?", p.ID); err != nil {
				return err
			}
		}
		for _, c := range p.Competitors {
			v, _ := json.Marshal(c)
			if _, err = tx.ExecContext(ctx, "INSERT INTO competitors(id,product_id,payload) VALUES(?,?,?)", c.ID, p.ID, v); err != nil {
				return err
			}
		}
		for _, im := range p.Images {
			v, _ := json.Marshal(im)
			if _, err = tx.ExecContext(ctx, "INSERT INTO product_images(id,product_id,payload) VALUES(?,?,?)", im.ID, p.ID, v); err != nil {
				return err
			}
		}
	}
	for _, j := range st.Jobs {
		v, _ := json.Marshal(j)
		if _, err = tx.ExecContext(ctx, "INSERT INTO jobs(id,kind,status,lease_until,payload,created_at) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status=VALUES(status),lease_until=VALUES(lease_until),payload=VALUES(payload)", j.ID, j.Kind, j.Status, j.LeaseUntil, v, j.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func dbQueries(conn db.DBTX) *db.Queries { return db.New(conn) }
