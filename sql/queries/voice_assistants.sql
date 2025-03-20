-- name: CreateVoiceAssistant :one
INSERT INTO voice_assistants (name, cloned_voice_id, user_id)
VALUES (
    ?,
    ?,
    ?
)
RETURNING *;


-- name: GetAssistantsByUserID :many
SELECT * FROM voice_assistants WHERE user_id = ?;
