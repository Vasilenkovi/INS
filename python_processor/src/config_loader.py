import yaml
from pathlib import Path
from pydantic import BaseModel, Field
from typing import List, Dict, Optional

class ProjectMeta(BaseModel):
    name: str
    language: str

class ForbiddenDependency(BaseModel):
    from_layer: str = Field(alias="from")
    to_layer: str = Field(alias="to")
    reason: str

class ArchitectureConfig(BaseModel):
    layers: List[str]
    forbidden_dependencies: List[ForbiddenDependency]

class SecurityConfig(BaseModel):
    forbid: List[str]
    require: List[str]

class AntiPattern(BaseModel):
    id: str
    description: str
    pattern: Optional[str] = None
    severity: str

class ResponseFormatConfig(BaseModel):
    type: str

class LLMReviewConfig(BaseModel):
    ignore_formatting: bool
    focus_on: List[str]
    require_fix_suggestions: bool
    response_format: ResponseFormatConfig

class CompleteUniversityStandard(BaseModel):
    version: float
    project: ProjectMeta
    severity_levels: List[str]
    review_priorities: Dict[str, float]
    architecture: ArchitectureConfig
    security: SecurityConfig
    anti_patterns: List[AntiPattern]
    llm_review: LLMReviewConfig

def load_standard(file_path: str | Path) -> CompleteUniversityStandard:
    with open(file_path, "r", encoding="utf-8") as f:
        raw_data = yaml.safe_load(f)
    return CompleteUniversityStandard(**raw_data)
