"""create_videos_table

Revision ID: fa11cd8a1aea
Revises:
Create Date: 2026-06-10 10:18:24.575603

"""
from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects.postgresql import UUID

revision: str = 'fa11cd8a1aea'
down_revision: Union[str, Sequence[str], None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'videos',
        sa.Column(
            'id',
            UUID(as_uuid=True),
            server_default=sa.text('gen_random_uuid()'),
            nullable=False,
        ),
        sa.Column('title', sa.String(length=255), nullable=False),
        # SeaweedFS (S3-compatible) object key, e.g. "videos/onboarding.mp4"
        sa.Column('file_path', sa.String(length=512), nullable=False),
        sa.Column(
            'views',
            sa.Integer(),
            server_default=sa.text('0'),
            nullable=False,
        ),
        sa.Column(
            'created_at',
            sa.DateTime(timezone=True),
            server_default=sa.text('now()'),
            nullable=False,
        ),
        sa.CheckConstraint('views >= 0', name='ck_videos_views_non_negative'),
        sa.PrimaryKeyConstraint('id'),
    )


def downgrade() -> None:
    op.drop_table('videos')
