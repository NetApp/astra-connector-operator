from enum import Enum


class Frequency(Enum):
    MINUTELY = "MINUTELY"
    HOURLY = "HOURLY"
    DAILY = "DAILY"


class AppmirrorState(Enum):
    PROMOTED = 'promoted'
    ESTABLISHED = 'established'
    COMPLETED = 'completed'
