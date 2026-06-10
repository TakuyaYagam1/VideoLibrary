from db.database import SessionLocal
from db.models import Video

SEED_VIDEOS = [
    {"title": "Вводный инструктаж", "file_path": "static/onboarding.mp4"},
    {"title": "Корпоративное совещание Q1", "file_path": "static/meeting.mp4"},
    {"title": "Техническая инструкция", "file_path": "static/instruction.mp4"},
    {"title": "Итоги квартала", "file_path": "static/quarterly_review.mp4"},
]


def seed() -> None:
    with SessionLocal() as session:
        existing_titles = {
            row[0] for row in session.query(Video.title).all()
        }
        new_videos = [
            Video(title=v["title"], file_path=v["file_path"])
            for v in SEED_VIDEOS
            if v["title"] not in existing_titles
        ]
        if new_videos:
            session.add_all(new_videos)
            session.commit()
            print(f"Добавлено {len(new_videos)} видео.")
        else:
            print("Данные уже заполнены, пропускаем.")


if __name__ == "__main__":
    seed()
