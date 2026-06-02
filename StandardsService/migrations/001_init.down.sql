-- 001_init.down.sql
ALTER TABLE code_standards DROP CONSTRAINT IF EXISTS fk_code_standards_active_version;
DROP TABLE IF EXISTS standard_versions;
DROP TABLE IF EXISTS code_standards;
DROP TABLE IF EXISTS repositories;
DROP TABLE IF EXISTS teams;
