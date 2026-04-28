from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any

EVALUATOR_VERSION = "2026-04-21.asb-metrics-v1"


@dataclass
class ExperimentRecord:
    run_id: str
    repeat_index: int
    benchmark_family: str
    benchmark_suite: str
    asb_attack: str
    case_id: str
    scenario: str
    agent_name: str
    agent_version: str
    defense: str
    under_attack: bool
    defense_enabled: bool
    attack_success: bool
    refused: bool
    task_success: bool
    benign_success: bool
    poison_detected: bool
    clean_detected_as_poison: bool
    latency_ms: int
    asr: float = 0.0
    asr_d: float = 0.0
    rr: float = 0.0
    pna: float = 0.0
    pna_d: float = 0.0
    bp: float = 0.0
    fnr: float = 0.0
    fpr: float = 0.0
    trace_path: str = ""
    raw_source_path: str = ""
    judge_method: str = "asb_original_output+mapping_review"
    evaluator_version: str = EVALUATOR_VERSION
    timestamp_utc: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    notes: str = ""

    def to_row(self) -> dict[str, Any]:
        return {
            "run_id": self.run_id,
            "repeat_index": self.repeat_index,
            "timestamp_utc": self.timestamp_utc,
            "benchmark_family": self.benchmark_family,
            "benchmark_suite": self.benchmark_suite,
            "asb_attack": self.asb_attack,
            "case_id": self.case_id,
            "scenario": self.scenario,
            "agent_name": self.agent_name,
            "agent_version": self.agent_version,
            "defense": self.defense,
            "under_attack": self.under_attack,
            "defense_enabled": self.defense_enabled,
            "attack_success": self.attack_success,
            "refused": self.refused,
            "task_success": self.task_success,
            "benign_success": self.benign_success,
            "poison_detected": self.poison_detected,
            "clean_detected_as_poison": self.clean_detected_as_poison,
            "latency_ms": self.latency_ms,
            "asr": self.asr,
            "asr_d": self.asr_d,
            "rr": self.rr,
            "pna": self.pna,
            "pna_d": self.pna_d,
            "bp": self.bp,
            "fnr": self.fnr,
            "fpr": self.fpr,
            "trace_path": self.trace_path,
            "raw_source_path": self.raw_source_path,
            "judge_method": self.judge_method,
            "evaluator_version": self.evaluator_version,
            "notes": self.notes,
        }


FIELDNAMES = list(
    ExperimentRecord(
        run_id="",
        repeat_index=0,
        benchmark_family="",
        benchmark_suite="",
        asb_attack="",
        case_id="",
        scenario="",
        agent_name="",
        agent_version="",
        defense="",
        under_attack=False,
        defense_enabled=False,
        attack_success=False,
        refused=False,
        task_success=False,
        benign_success=False,
        poison_detected=False,
        clean_detected_as_poison=False,
        latency_ms=0,
    ).to_row().keys()
)
