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
			CREATE TYPE trip_status AS ENUM ('OK', 'ROUTE_VIOLATION', 'MISMATCH_PLATE', 'NO_ASSIGNMENT', 'SUSPICIOUS_VOLUME');
		ELSE
			-- Если тип уже существует, но имеет неправильные значения, пересоздаем его
			-- (только если таблица trips пустая или не существует)
				IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'OK' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'trip_status')) THEN
					-- Проверяем, есть ли данные в trips
					IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'trips') OR
					   (EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'trips') AND NOT EXISTS (SELECT 1 FROM trips LIMIT 1)) THEN
						-- Таблица пустая или не существует - можно пересоздать ENUM
						-- Сохраняем информацию о таблице перед удалением
						DROP TYPE IF EXISTS trip_status CASCADE;
						CREATE TYPE trip_status AS ENUM ('OK', 'ROUTE_VIOLATION', 'MISMATCH_PLATE', 'NO_ASSIGNMENT', 'SUSPICIOUS_VOLUME');
						-- Таблица trips будет пересоздана позже в миграции
					ELSE
					-- Таблица не пустая - добавляем новые значения
					IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'OK' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'trip_status')) THEN
						ALTER TYPE trip_status ADD VALUE 'OK';
					END IF;
					IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'ROUTE_VIOLATION' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'trip_status')) THEN
						ALTER TYPE trip_status ADD VALUE 'ROUTE_VIOLATION';
					END IF;
					IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'MISMATCH_PLATE' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'trip_status')) THEN
						ALTER TYPE trip_status ADD VALUE 'MISMATCH_PLATE';
					END IF;
					IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'NO_ASSIGNMENT' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'trip_status')) THEN
						ALTER TYPE trip_status ADD VALUE 'NO_ASSIGNMENT';
					END IF;
					IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'SUSPICIOUS_VOLUME' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'trip_status')) THEN
						ALTER TYPE trip_status ADD VALUE 'SUSPICIOUS_VOLUME';
					END IF;
				END IF;
			END IF;
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'appeal_status') THEN
			CREATE TYPE appeal_status AS ENUM ('SUBMITTED', 'UNDER_REVIEW', 'NEED_INFO', 'APPROVED', 'REJECTED', 'CLOSED');
		ELSE
			-- Если тип уже существует, добавляем SUBMITTED если его нет
			IF NOT EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'SUBMITTED' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'appeal_status')) THEN
				ALTER TYPE appeal_status ADD VALUE 'SUBMITTED' BEFORE 'UNDER_REVIEW';
			END IF;
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'driver_mark_status') THEN
			CREATE TYPE driver_mark_status AS ENUM ('NOT_STARTED', 'IN_WORK', 'COMPLETED');
		END IF;
	END
	$$;`,
	`CREATE TABLE IF NOT EXISTS tickets (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		cleaning_area_id UUID NOT NULL,
		contractor_id UUID NOT NULL,
		contract_id UUID,
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
	`DO $$
	BEGIN
		-- Добавляем contract_id если его нет (nullable, так как старые записи могут не иметь его)
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'tickets' AND column_name = 'contract_id') THEN
			ALTER TABLE tickets ADD COLUMN contract_id UUID;
		END IF;
	END
	$$;`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_cleaning_area_id ON tickets (cleaning_area_id);`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_contractor_id ON tickets (contractor_id);`,
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'tickets' AND column_name = 'contract_id'
		) THEN
			IF EXISTS (SELECT 1 FROM tickets WHERE contract_id IS NULL LIMIT 1) THEN
				RAISE EXCEPTION 'tickets.contract_id contains NULL values; assign contracts before running migration';
			END IF;
			ALTER TABLE tickets ALTER COLUMN contract_id SET NOT NULL;
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'contracts') AND
		   EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'tickets' AND column_name = 'contract_id') THEN
			ALTER TABLE tickets DROP CONSTRAINT IF EXISTS fk_tickets_contract;
			ALTER TABLE tickets
				ADD CONSTRAINT fk_tickets_contract
				FOREIGN KEY (contract_id)
				REFERENCES contracts(id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT;
		END IF;
	END
	$$;`,
	`DO $$
	BEGIN
		-- Создаем индекс на contract_id только если колонка существует
		IF EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'tickets' AND column_name = 'contract_id') THEN
			CREATE INDEX IF NOT EXISTS idx_tickets_contract_id ON tickets (contract_id);
		END IF;
	END
	$$;`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_created_by_org_id ON tickets (created_by_org_id);`,
	`CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets (status);`,
	`CREATE TABLE IF NOT EXISTS ticket_assignments (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
		driver_id UUID NOT NULL,
		vehicle_id UUID NOT NULL,
		driver_mark_status driver_mark_status NOT NULL DEFAULT 'NOT_STARTED',
		assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		unassigned_at TIMESTAMPTZ,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`DO $$
	BEGIN
		-- Добавляем driver_mark_status если его нет
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'ticket_assignments' AND column_name = 'driver_mark_status') THEN
			ALTER TABLE ticket_assignments ADD COLUMN driver_mark_status driver_mark_status NOT NULL DEFAULT 'NOT_STARTED';
		END IF;
	END
	$$;`,
	`CREATE INDEX IF NOT EXISTS idx_ticket_assignments_ticket_id ON ticket_assignments (ticket_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ticket_assignments_driver_id ON ticket_assignments (driver_id);`,
	`CREATE INDEX IF NOT EXISTS idx_ticket_assignments_vehicle_id ON ticket_assignments (vehicle_id);`,
	`CREATE TABLE IF NOT EXISTS trips (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL,
		ticket_assignment_id UUID REFERENCES ticket_assignments(id) ON DELETE SET NULL,
		driver_id UUID,
		vehicle_id UUID,
		camera_id UUID,
		polygon_id UUID,
		vehicle_plate_number VARCHAR(32),
		detected_plate_number VARCHAR(32),
		entry_lpr_event_id UUID,
		exit_lpr_event_id UUID,
		entry_volume_event_id UUID,
		exit_volume_event_id UUID,
		detected_volume_entry DOUBLE PRECISION,
		detected_volume_exit DOUBLE PRECISION,
		entry_at TIMESTAMPTZ NOT NULL,
		exit_at TIMESTAMPTZ,
		status trip_status NOT NULL DEFAULT 'OK',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`DO $$
	BEGIN
		-- Добавляем недостающие поля в trips если их нет
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'driver_id') THEN
			ALTER TABLE trips ADD COLUMN driver_id UUID;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'vehicle_id') THEN
			ALTER TABLE trips ADD COLUMN vehicle_id UUID;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'entry_lpr_event_id') THEN
			ALTER TABLE trips ADD COLUMN entry_lpr_event_id UUID;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'exit_lpr_event_id') THEN
			ALTER TABLE trips ADD COLUMN exit_lpr_event_id UUID;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'entry_volume_event_id') THEN
			ALTER TABLE trips ADD COLUMN entry_volume_event_id UUID;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'exit_volume_event_id') THEN
			ALTER TABLE trips ADD COLUMN exit_volume_event_id UUID;
		END IF;
	END
	$$;`,
	`CREATE INDEX IF NOT EXISTS idx_trips_ticket_id ON trips (ticket_id);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_ticket_assignment_id ON trips (ticket_assignment_id);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_driver_id ON trips (driver_id);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_vehicle_id ON trips (vehicle_id);`,
	`CREATE INDEX IF NOT EXISTS idx_trips_entry_at ON trips (entry_at);`,
	`DO $$
	BEGIN
		-- Создаем индекс на status только если колонка существует
		IF EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'trips' AND column_name = 'status') THEN
			CREATE INDEX IF NOT EXISTS idx_trips_status ON trips (status);
		END IF;
	END
	$$;`,
	`CREATE TABLE IF NOT EXISTS lpr_events (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		camera_id UUID NOT NULL,
		polygon_id UUID,
		plate_number VARCHAR(32) NOT NULL,
		detected_at TIMESTAMPTZ NOT NULL,
		direction VARCHAR(20),
		confidence DOUBLE PRECISION,
		photo_url TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`DO $$
	BEGIN
		-- Добавляем photo_url если его нет
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'lpr_events' AND column_name = 'photo_url') THEN
			ALTER TABLE lpr_events ADD COLUMN photo_url TEXT;
		END IF;
	END
	$$;`,
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
		photo_url TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`DO $$
	BEGIN
		-- Добавляем photo_url если его нет
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'volume_events' AND column_name = 'photo_url') THEN
			ALTER TABLE volume_events ADD COLUMN photo_url TEXT;
		END IF;
	END
	$$;`,
	`CREATE INDEX IF NOT EXISTS idx_volume_events_camera_id ON volume_events (camera_id);`,
	`CREATE INDEX IF NOT EXISTS idx_volume_events_detected_at ON volume_events (detected_at);`,
	`CREATE TABLE IF NOT EXISTS appeals (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		trip_id UUID REFERENCES trips(id) ON DELETE CASCADE,
		ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,
		created_by_user_id UUID NOT NULL,
		status appeal_status NOT NULL DEFAULT 'NEED_INFO',
		reason TEXT NOT NULL,
		appeal_reason_type VARCHAR(50),
		comment TEXT NOT NULL,
		admin_response TEXT,
		resolved_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`DO $$
	BEGIN
		-- Добавляем недостающие поля в appeals если их нет
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'appeals' AND column_name = 'appeal_reason_type') THEN
			ALTER TABLE appeals ADD COLUMN appeal_reason_type VARCHAR(50);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'appeals' AND column_name = 'comment') THEN
			-- Если comment нет, добавляем его (может быть nullable для старых записей)
			ALTER TABLE appeals ADD COLUMN comment TEXT;
			-- Обновляем существующие записи
			UPDATE appeals SET comment = reason WHERE comment IS NULL;
			-- Теперь делаем NOT NULL
			ALTER TABLE appeals ALTER COLUMN comment SET NOT NULL;
		END IF;
		-- Обновляем дефолтный статус на SUBMITTED если тип поддерживает это значение
		IF EXISTS (SELECT 1 FROM pg_enum WHERE enumlabel = 'SUBMITTED' AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'appeal_status')) THEN
			ALTER TABLE appeals ALTER COLUMN status SET DEFAULT 'SUBMITTED';
		END IF;
	END
	$$;`,
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
