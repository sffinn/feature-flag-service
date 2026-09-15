CREATE TABLE IF NOT EXISTS global_flags (
    flagname   TEXT PRIMARY KEY,
    enabled    BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_flags (
    username   TEXT NOT NULL,
    flagname   TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (username, flagname)
);
