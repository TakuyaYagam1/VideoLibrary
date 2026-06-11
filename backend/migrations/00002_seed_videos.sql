-- +goose Up
INSERT INTO videos (id, title, file_path, views, created_at)
VALUES
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1001',
        'Вводный курс по безопасности',
        'http://localhost:8888/videos/security-onboarding.mp4',
        0,
        now()
    ),
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1002',
        'Регламент работы с клиентскими данными',
        'http://localhost:8888/videos/customer-data-policy.mp4',
        0,
        now()
    ),
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1003',
        'Запись ежемесячного собрания',
        'http://localhost:8888/videos/monthly-meeting.mp4',
        0,
        now()
    ),
    (
        '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1004',
        'Инструкция по внутренним инструментам',
        'http://localhost:8888/videos/internal-tools-guide.mp4',
        0,
        now()
    )
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM videos
WHERE id IN (
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1001',
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1002',
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1003',
    '01978a7a-8a40-7a0d-9b2f-6f0c1e5f1004'
);
