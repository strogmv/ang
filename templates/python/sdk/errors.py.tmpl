"""Error wrappers for ANG Python SDK."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class ProblemDetails(BaseModel):
    type: str = "about:blank"
    title: str = "Error"
    status: int | None = None
    detail: str | None = None
    instance: str | None = None
    extensions: dict[str, Any] = Field(default_factory=dict)


class AngAPIError(Exception):
    def __init__(
        self,
        status_code: int,
        message: str,
        problem: ProblemDetails | None = None,
        response_text: str | None = None,
    ) -> None:
        super().__init__(message)
        self.status_code = status_code
        self.problem = problem
        self.response_text = response_text
