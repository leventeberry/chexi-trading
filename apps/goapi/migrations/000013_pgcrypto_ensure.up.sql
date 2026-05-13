-- Belt-and-suspenders: gen_random_uuid() requires pgcrypto (already enabled in 000001).
-- Safe on fresh DBs (IF NOT EXISTS) and for databases upgrading from v12 without touching 000001.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
