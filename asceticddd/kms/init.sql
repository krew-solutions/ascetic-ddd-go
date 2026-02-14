CREATE TABLE kms_keys (
    tenant_id varchar(128) NOT NULL,
    key_version integer NOT NULL,
    encrypted_kek bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT kms_keys_pk PRIMARY KEY (tenant_id, key_version)
);
