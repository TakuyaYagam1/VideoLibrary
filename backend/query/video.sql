-- name: ListVideos :many
SELECT
    id,
    title,
    file_path,
    views,
    created_at
FROM videos
ORDER BY created_at DESC, id;

-- name: GetVideoByID :one
SELECT
    id,
    title,
    file_path,
    views,
    created_at
FROM videos
WHERE id = $1;

-- name: IncrementViews :one
WITH updated_video AS (
    UPDATE videos
    SET views = videos.views + 1
    WHERE videos.id = sqlc.arg(video_id)
    RETURNING videos.id, videos.views
),
inserted_event AS (
    INSERT INTO outbox_events (
        id,
        event_type,
        payload
    )
    SELECT
        sqlc.arg(outbox_event_id)::uuid,
        'cache.invalidate_videos_list',
        jsonb_build_object(
            'video_id', updated_video.id,
            'views', updated_video.views
        )
    FROM updated_video
    RETURNING id
)
SELECT updated_video.views
FROM updated_video
JOIN inserted_event ON true;
