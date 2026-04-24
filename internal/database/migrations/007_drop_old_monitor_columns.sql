-- +goose Up
ALTER TABLE monitors
    DROP COLUMN url,
    DROP COLUMN method,
    DROP COLUMN expected_status,
    DROP COLUMN keyword;
