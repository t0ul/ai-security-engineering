"""Tool plugin contract — one narrow, scoped capability per tool."""
from enum import Enum
from dataclasses import dataclass, field
from typing import List, Dict


class Capability(Enum):
    READ_ONLY = "read_only"      # may only produce text (digest, action-items, contacts)
    WRITE_ICS = "write_ics"      # may emit an inert .ics artifact for the user to accept


@dataclass
class ToolResult:
    tool: str
    capability: Capability
    events: List = field(default_factory=list)      # list[Event] for extractors
    text: str = ""                                   # for read-only text tools
    artifacts: Dict[str, str] = field(default_factory=dict)  # filename -> content (write tools only)
    warnings: List[str] = field(default_factory=list)


class Tool:
    """Subclass, set name + capability, implement run()."""
    name: str = "base"
    capability: Capability = Capability.READ_ONLY

    def run(self, email_text: str, ctx: dict) -> ToolResult:
        raise NotImplementedError


_REGISTRY: Dict[str, Tool] = {}


def register(tool: Tool) -> Tool:
    _REGISTRY[tool.name] = tool
    return tool


def get_tool(name: str) -> Tool:
    return _REGISTRY[name]


def all_tools() -> Dict[str, Tool]:
    return dict(_REGISTRY)
