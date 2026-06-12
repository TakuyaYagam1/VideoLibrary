-- +goose Up
INSERT INTO videos (id, title, file_path, views, created_at)
VALUES
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1001',
        'Planet 1.5 MB',
        '/videos/planet_1.5mb.mp4',
        0,
        now()
    ),
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1002',
        'Planet 3 MB',
        '/videos/planet_3mb.mp4',
        0,
        now()
    ),
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1003',
        'Planet 10 MB',
        '/videos/planet_10mb.mp4',
        0,
        now()
    ),
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1004',
        'Planet 18 MB',
        '/videos/planet_18mb.mp4',
        0,
        now()
    )
ON CONFLICT (id) DO UPDATE
SET title = EXCLUDED.title,
    file_path = EXCLUDED.file_path;

-- +goose Down
DELETE FROM videos
WHERE id IN (
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1001',
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1002',
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1003',
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1004'
);
