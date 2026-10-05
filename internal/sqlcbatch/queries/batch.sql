-- name: ExecuteValues :batchexec
SELECT $1::integer;

-- name: ListValues :batchmany
SELECT generate_series(1, $1::integer)::integer AS value;

-- name: GetValues :batchone
SELECT $1::integer AS value;
