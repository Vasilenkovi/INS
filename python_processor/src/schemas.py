from pydantic import BaseModel

class AnalysisRequest(BaseModel):
    project_id: int
    commit_sha: str
    file_path: str
    code_content: str
