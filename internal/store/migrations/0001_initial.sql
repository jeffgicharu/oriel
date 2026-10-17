-- Oriel's data. Times are Unix seconds (UTC); calendar dates are 'YYYY-MM-DD'
-- in Nairobi time; money is whole Kenyan shillings.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

CREATE TABLE staff (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('manager', 'agent')),
    title         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    colour        TEXT NOT NULL
) STRICT;

CREATE TABLE staff_sessions (
    token_hash TEXT PRIMARY KEY,
    staff_id   INTEGER NOT NULL REFERENCES staff (id),
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;

-- Every published price list. The newest one is in force.
CREATE TABLE price_lists (
    id           INTEGER PRIMARY KEY,
    published_at INTEGER NOT NULL,
    published_by INTEGER REFERENCES staff (id),
    floor_bp     INTEGER NOT NULL,
    river_bp     INTEGER NOT NULL,
    base_a       INTEGER NOT NULL,
    base_b       INTEGER NOT NULL,
    base_c       INTEGER NOT NULL,
    base_d       INTEGER NOT NULL,
    base_p       INTEGER NOT NULL,
    note         TEXT NOT NULL DEFAULT ''
) STRICT;

-- The state of each home. Its type, floor and position come from the code.
CREATE TABLE units (
    id                TEXT PRIMARY KEY,
    status            TEXT NOT NULL CHECK (status IN ('available', 'held', 'reserved', 'sold', 'withheld')),
    price_override    INTEGER,
    status_changed_at INTEGER NOT NULL
) STRICT;

CREATE TABLE unit_events (
    id         INTEGER PRIMARY KEY,
    unit_id    TEXT NOT NULL REFERENCES units (id),
    at         INTEGER NOT NULL,
    staff_id   INTEGER REFERENCES staff (id),
    kind       TEXT NOT NULL CHECK (kind IN ('status', 'price')),
    from_value TEXT NOT NULL DEFAULT '',
    to_value   TEXT NOT NULL DEFAULT '',
    reason     TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX unit_events_unit ON unit_events (unit_id, at);

CREATE TABLE leads (
    id               INTEGER PRIMARY KEY,
    name             TEXT NOT NULL,
    email            TEXT NOT NULL,
    phone            TEXT NOT NULL,
    source           TEXT NOT NULL,
    stage            TEXT NOT NULL CHECK (stage IN ('new', 'contacted', 'viewing', 'reservation', 'won', 'lost')),
    agent_id         INTEGER REFERENCES staff (id),
    interest         TEXT NOT NULL DEFAULT '',
    budget           INTEGER,
    buying_for       TEXT NOT NULL DEFAULT '',
    follow_up_on     TEXT,
    lost_reason      TEXT NOT NULL DEFAULT '',
    sort_order       REAL NOT NULL DEFAULT 0,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    stage_changed_at INTEGER NOT NULL
) STRICT;
CREATE INDEX leads_stage ON leads (stage, sort_order);
CREATE INDEX leads_agent ON leads (agent_id);
CREATE INDEX leads_email ON leads (email);

CREATE TABLE lead_events (
    id       INTEGER PRIMARY KEY,
    lead_id  INTEGER NOT NULL REFERENCES leads (id) ON DELETE CASCADE,
    at       INTEGER NOT NULL,
    staff_id INTEGER REFERENCES staff (id),
    kind     TEXT NOT NULL,
    body     TEXT NOT NULL DEFAULT '',
    unit_id  TEXT
) STRICT;
CREATE INDEX lead_events_lead ON lead_events (lead_id, at);

CREATE TABLE viewings (
    id            INTEGER PRIMARY KEY,
    ref           TEXT NOT NULL UNIQUE,
    token         TEXT NOT NULL UNIQUE,
    lead_id       INTEGER NOT NULL REFERENCES leads (id),
    unit_id       TEXT,
    starts_at     INTEGER NOT NULL,
    party_size    INTEGER NOT NULL DEFAULT 1,
    status        TEXT NOT NULL CHECK (status IN ('booked', 'attended', 'no_show', 'cancelled')),
    notes         TEXT NOT NULL DEFAULT '',
    outcome_notes TEXT NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;
CREATE INDEX viewings_starts ON viewings (starts_at);
CREATE INDEX viewings_lead ON viewings (lead_id);

-- Days the sales suite is closed for viewings.
CREATE TABLE viewing_blocks (
    day    TEXT PRIMARY KEY,
    reason TEXT NOT NULL
) STRICT;

CREATE TABLE reservations (
    id                  INTEGER PRIMARY KEY,
    ref                 TEXT NOT NULL UNIQUE,
    token               TEXT NOT NULL UNIQUE,
    lead_id             INTEGER NOT NULL REFERENCES leads (id),
    unit_id             TEXT NOT NULL REFERENCES units (id),
    plan                TEXT NOT NULL,
    price               INTEGER NOT NULL,
    status              TEXT NOT NULL CHECK (status IN ('requested', 'confirmed', 'declined', 'lapsed', 'completed', 'cancelled')),
    requested_at        INTEGER NOT NULL,
    hold_expires_at     INTEGER NOT NULL,
    decided_at          INTEGER,
    decided_by          INTEGER REFERENCES staff (id),
    decline_reason      TEXT NOT NULL DEFAULT '',
    offer_number        TEXT,
    deposit_received_at INTEGER,
    deposit_amount      INTEGER,
    buyer_note          TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX reservations_unit ON reservations (unit_id, status);
CREATE INDEX reservations_status ON reservations (status, requested_at);

CREATE TABLE activity (
    id       INTEGER PRIMARY KEY,
    at       INTEGER NOT NULL,
    staff_id INTEGER REFERENCES staff (id),
    actor    TEXT NOT NULL CHECK (actor IN ('staff', 'visitor', 'system')),
    action   TEXT NOT NULL,
    subject  TEXT NOT NULL,
    detail   TEXT NOT NULL DEFAULT '',
    link     TEXT NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX activity_at ON activity (at);
