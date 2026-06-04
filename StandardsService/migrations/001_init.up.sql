-- =============================================================================
-- 001_init.up.sql
-- Начальная схема БД для standards-service
-- =============================================================================

-- -----------------------------------------------------------------------------
-- teams
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS teams (
    id          UUID PRIMARY KEY,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    created_by  TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_teams_slug ON teams(slug);

-- -----------------------------------------------------------------------------
-- repositories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS repositories (
    id          UUID        PRIMARY KEY,
    team_id     UUID        NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    gitlab_id   INTEGER     NOT NULL UNIQUE,  -- GitLab project ID, уникален глобально
    name        TEXT        NOT NULL,
    full_path   TEXT        NOT NULL DEFAULT '',
    added_by    TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_repositories_team_id   ON repositories(team_id);
CREATE INDEX IF NOT EXISTS idx_repositories_gitlab_id ON repositories(gitlab_id);

-- -----------------------------------------------------------------------------
-- code_standards
-- Один стандарт на команду.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS code_standards (
    id                UUID        PRIMARY KEY,
    team_id           UUID        NOT NULL UNIQUE REFERENCES teams(id) ON DELETE CASCADE,
    active_version_id UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- -----------------------------------------------------------------------------
-- standard_versions
-- Иммутабельные версии стандарта. Новая загрузка = новая запись.
-- Новая версия автоматически становится активной.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS standard_versions (
    id               UUID        PRIMARY KEY,
    code_standard_id UUID        NOT NULL REFERENCES code_standards(id) ON DELETE CASCADE,
    version          INTEGER     NOT NULL,
    custom_rules     TEXT        NOT NULL DEFAULT '',
    comment          TEXT        NOT NULL DEFAULT '',
    created_by       TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (code_standard_id, version)
);

CREATE INDEX IF NOT EXISTS idx_standard_versions_standard_id ON standard_versions(code_standard_id);

-- Отложенный FK: code_standards.active_version_id → standard_versions.id
ALTER TABLE code_standards
    ADD CONSTRAINT fk_code_standards_active_version
    FOREIGN KEY (active_version_id)
    REFERENCES standard_versions(id)
    ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;
