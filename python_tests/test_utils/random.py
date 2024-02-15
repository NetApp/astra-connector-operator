import uuid


def get_short_uuid() -> str:
    return str(uuid.uuid4())[:5]
