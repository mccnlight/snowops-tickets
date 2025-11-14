package db

import (
	"fmt"

	"gorm.io/gorm"
)

var migrationStatements = []string{
	`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
	`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`,
	// PostGIS is pre-installed in postgis/postgis image, so we don't need to create it
	// If using standard postgres image, uncomment the line below:
	// `CREATE EXTENSION IF NOT EXISTS "postgis";`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ticket_status') THEN
			CREATE TYPE ticket_status AS ENUM ('PLANNED', 'IN_PROGRESS', 'COMPLETED', 'CLOSED', 'CANCELLED');
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'trip_status') THEN
			CREATE TYPE trip_status AS ENUM ('PENDING', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED');
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'appeal_status') THEN
			CREATE TYPE appeal_status AS ENUM ('NEED_INFO', 'UNDER_REVIEW', 'APPROVED', 'REJECTED', 'CLOSED');
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'violation_type') THEN
			CREATE TYPE violation_type AS ENUM ('ROUTE_VIOLATION', 'MISMATCH_PLATE', 'NO_ASSIGNMENT', 'SUSPICIOUS_VOLUME');
		END IF;
	END
	$$;`,
	`CREATE TABLE IF NOT EXISTS tickets (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		cleaning_area_id UUID NOT NULL,
		contractor_id UUID NOT NULL,
		created_by_org_id UUID NOT NULL,
		status ticket_status NOT NULL DEFAULT 'PLANNED',
		planned_start_at TIMESTAMPTZ NOT NULL,
		planned_end_at TIMESTAMPTZ NOT NULL,
		fact_start_at TIMESTAMPTZ,
		fact_end_at TIMESTAMPTZ,
		description TEXT,
		photo_url TEXT,
		latitude DOUBLE PRECISION,
		longitude DOUBLE PRECISION,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_cleaning_area_id ON tickets (cleaning_area_id);`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_contractor_id ON tickets (contractor_id);`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_created_by_org_id ON tickets (created_by_org_id);`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets (status);`,
	`CREATE TABLE IF NOT EXISTS ticket_assignments (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
		driver_id UUID NOT NULL,
		vehicle_id UUID NOT NULL,
		assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		unassigned_at TIMESTAMPTZ,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_ticket_assignments_ticket_id ON ticket_assignments (ticket_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ticket_assignments_driver_id ON ticket_assignments (driver_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ticket_assignments_vehicle_id ON ticket_assignments (vehicle_id);`,
	`CREATE TABLE IF NOT EXISTS trips (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL,
		ticket_assignment_id UUID REFERENCES ticket_assignments(id) ON DELETE SET NULL,
		camera_id UUID,
		polygon_id UUID,
		vehicle_plate_number VARCHAR(32),
		detected_plate_number VARCHAR(32),
		detected_volume_entry DOUBLE PRECISION,
		detected_volume_exit DOUBLE PRECISION,
		entry_at TIMESTAMPTZ NOT NULL,
		exit_at TIMESTAMPTZ,
		status trip_status NOT NULL DEFAULT 'PENDING',
		has_violations BOOLEAN NOT NULL DEFAULT FALSE,
		violation_type violation_type,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_ticket_id ON trips (ticket_id);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_ticket_assignment_id ON trips (ticket_assignment_id);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_entry_at ON trips (entry_at);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_status ON trips (status);`,
	`CREATE TABLE IF NOT EXISTS lpr_events (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		camera_id UUID NOT NULL,
		polygon_id UUID,
		plate_number VARCHAR(32) NOT NULL,
		detected_at TIMESTAMPTZ NOT NULL,
		direction VARCHAR(20),
		confidence DOUBLE PRECISION,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_lpr_events_camera_id ON lpr_events (camera_id);`,
	`CREATE INDEX IF NOT EXISTS idx_lpr_events_detected_at ON lpr_events (detected_at);`,
	`CREATE INDEX IF NOT EXISTS idx_lpr_events_plate_number ON lpr_events (plate_number);`,
	`CREATE TABLE IF NOT EXISTS volume_events (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		camera_id UUID NOT NULL,
		polygon_id UUID,
		detected_volume DOUBLE PRECISION NOT NULL,
		detected_at TIMESTAMPTZ NOT NULL,
		direction VARCHAR(20),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_volume_events_camera_id ON volume_events (camera_id);`,
	`CREATE INDEX IF NOT EXISTS idx_volume_events_detected_at ON volume_events (detected_at);`,
	`CREATE TABLE IF NOT EXISTS appeals (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		trip_id UUID REFERENCES trips(id) ON DELETE CASCADE,
		ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,
		created_by_user_id UUID NOT NULL,
		status appeal_status NOT NULL DEFAULT 'NEED_INFO',
		reason TEXT NOT NULL,
		admin_response TEXT,
		resolved_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_appeals_trip_id ON appeals (trip_id);`,
	`CREATE INDEX IF NOT EXISTS idx_appeals_ticket_id ON appeals (ticket_id);`,
	`CREATE INDEX IF NOT EXISTS idx_appeals_status ON appeals (status);`,
	`CREATE TABLE IF NOT EXISTS appeal_comments (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		appeal_id UUID NOT NULL REFERENCES appeals(id) ON DELETE CASCADE,
		created_by_user_id UUID NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_appeal_comments_appeal_id ON appeal_comments (appeal_id);`,
	`CREATE OR REPLACE FUNCTION set_updated_at()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = NOW();
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_tickets_updated_at') THEN
			CREATE TRIGGER trg_tickets_updated_at
				BEFORE UPDATE ON tickets
				FOR EACH ROW
				EXECUTE PROCEDURE set_updated_at();
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_ticket_assignments_updated_at') THEN
			CREATE TRIGGER trg_ticket_assignments_updated_at
				BEFORE UPDATE ON ticket_assignments
				FOR EACH ROW
				EXECUTE PROCEDURE set_updated_at();
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_trips_updated_at') THEN
			CREATE TRIGGER trg_trips_updated_at
				BEFORE UPDATE ON trips
				FOR EACH ROW
				EXECUTE PROCEDURE set_updated_at();
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_appeals_updated_at') THEN
			CREATE TRIGGER trg_appeals_updated_at
				BEFORE UPDATE ON appeals
				FOR EACH ROW
				EXECUTE PROCEDURE set_updated_at();
		END IF;
	END
	$$;`,
}

func runMigrations(db *gorm.DB) error {
	for i, stmt := range migrationStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}

