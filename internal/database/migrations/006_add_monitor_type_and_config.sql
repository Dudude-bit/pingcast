ALTER TABLE monitors
    ADD COLUMN type VARCHAR(10) NOT NULL DEFAULT 'http',
    ADD COLUMN check_config JSONB NOT NULL DEFAULT '{}';

UPDATE monitors SET check_config = jsonb_strip_nulls(jsonb_build_object(
    'url', url,
    'method', method,
    'expected_status', expected_status,
    'keyword', keyword
));
