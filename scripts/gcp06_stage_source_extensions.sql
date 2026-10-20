-- Recreate extensions observed on the local source but not declared by V1-V61.
-- This file is applied only to the disposable PostgreSQL staging database;
-- migration history V1-V64 remains byte-for-byte unchanged.
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;
CREATE EXTENSION IF NOT EXISTS postgis_tiger_geocoder;
CREATE EXTENSION IF NOT EXISTS postgis_topology;
