-- name: CreateClients :one
INSERT INTO clients (id, name, email, phone, address, agent_id)
VALUES(
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
)RETURNING *;

-- name: GetClientById :one
SELECT * FROM clients WHERE id = ?;

-- name: GetClientByEmail :one
SELECT * FROM clients WHERE email = ?;

-- name: GetAgentClients :many
SELECT * FROM clients WHERE agent_id = ?;

