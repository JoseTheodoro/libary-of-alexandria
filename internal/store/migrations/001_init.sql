CREATE TABLE sources (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  authors_json TEXT NOT NULL,
  edition TEXT,
  format TEXT NOT NULL,
  permission_json TEXT NOT NULL
);

CREATE TABLE source_revisions (
  source_id TEXT NOT NULL REFERENCES sources(id),
  sha256 TEXT NOT NULL,
  PRIMARY KEY (source_id, sha256)
);

CREATE TABLE extraction_revisions (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL,
  source_sha256 TEXT NOT NULL,
  events_sha256 TEXT NOT NULL,
  contract_version INTEGER NOT NULL,
  engine_json TEXT NOT NULL,
  mode TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('complete', 'partial')),
  UNIQUE (source_id, id),
  FOREIGN KEY (source_id, source_sha256) REFERENCES source_revisions(source_id, sha256)
);

CREATE TABLE document_units (
  extraction_id TEXT NOT NULL REFERENCES extraction_revisions(id),
  unit_id TEXT NOT NULL,
  ordinal INTEGER NOT NULL CHECK (ordinal > 0),
  text TEXT NOT NULL,
  locator_json TEXT NOT NULL,
  PRIMARY KEY (extraction_id, unit_id),
  UNIQUE (extraction_id, ordinal)
);

CREATE TABLE sections (
  extraction_id TEXT NOT NULL REFERENCES extraction_revisions(id),
  ordinal INTEGER NOT NULL CHECK (ordinal > 0),
  title TEXT NOT NULL,
  depth INTEGER NOT NULL CHECK (depth >= 0),
  locator_json TEXT NOT NULL,
  PRIMARY KEY (extraction_id, ordinal)
);

CREATE TABLE diagnostics (
  extraction_id TEXT NOT NULL REFERENCES extraction_revisions(id),
  ordinal INTEGER NOT NULL CHECK (ordinal > 0),
  code TEXT NOT NULL,
  severity TEXT NOT NULL,
  message TEXT NOT NULL,
  locator_json TEXT,
  PRIMARY KEY (extraction_id, ordinal)
);

CREATE TABLE chunk_profiles (
  id TEXT PRIMARY KEY,
  config_json TEXT NOT NULL
);

CREATE TABLE fragments (
  id INTEGER PRIMARY KEY,
  stable_id TEXT NOT NULL UNIQUE,
  extraction_id TEXT NOT NULL,
  unit_id TEXT NOT NULL,
  profile_id TEXT NOT NULL REFERENCES chunk_profiles(id),
  start_byte INTEGER NOT NULL CHECK (start_byte >= 0),
  end_byte INTEGER NOT NULL CHECK (end_byte > start_byte),
  FOREIGN KEY (extraction_id, unit_id) REFERENCES document_units(extraction_id, unit_id),
  UNIQUE (extraction_id, unit_id, profile_id, start_byte, end_byte)
);

CREATE TABLE active_documents (
  source_id TEXT PRIMARY KEY REFERENCES sources(id),
  extraction_id TEXT NOT NULL,
  profile_id TEXT NOT NULL REFERENCES chunk_profiles(id),
  FOREIGN KEY (source_id, extraction_id) REFERENCES extraction_revisions(source_id, id)
);

CREATE TABLE index_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  version INTEGER NOT NULL,
  config_json TEXT NOT NULL,
  active_digest TEXT NOT NULL
);

-- A projeção é derivada. Seu rowid corresponde à chave inteira em fragments.
CREATE VIRTUAL TABLE fragment_fts USING fts5(
  title,
  body,
  tokenize = 'unicode61 remove_diacritics 0'
);
