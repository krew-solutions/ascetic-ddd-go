CREATE TABLE event_log (
    tenant_id varchar(128) NOT NULL,  -- integer NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    stream_type varchar(128) NOT NULL,
    stream_id jsonb NOT NULL,
    stream_position integer NOT NULL,
    event_type varchar(60) NOT NULL,
    event_version smallint NOT NULL,
    payload bytea NOT NULL,
    metadata jsonb NULL,
    CONSTRAINT event_log_pk PRIMARY KEY (tenant_id, stream_type, stream_id, stream_position)
);
CREATE UNIQUE INDEX event_log__event_id_uniq ON event_log( ((metadata->>'event_id')::uuid) );

CREATE TABLE IF NOT EXISTS stream_deks (
    tenant_id varchar(128) NOT NULL,
    stream_type varchar(128) NOT NULL,
    stream_id jsonb NOT NULL,
    version integer NOT NULL,
    encrypted_dek bytea NOT NULL,
    algorithm varchar(32) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT stream_deks_pk PRIMARY KEY (tenant_id, stream_type, stream_id, version)
);
