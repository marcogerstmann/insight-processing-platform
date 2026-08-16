import pytest

from ipp_ai.application.secrets import resolve_secret


class _FakeSecretProvider:
    """Satisfies ports.SecretProvider structurally — no inheritance, no import of ports."""

    def __init__(self, value: str) -> None:
        self._value = value

    def get(self, name: str) -> str:
        return self._value


def test_resolve_secret_unset(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("TEST_SECRET", raising=False)
    assert resolve_secret("TEST_SECRET", None) == ""


def test_resolve_secret_literal(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TEST_SECRET", "sk-ant-abc")
    assert resolve_secret("TEST_SECRET", None) == "sk-ant-abc"


def test_resolve_secret_ssm(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TEST_SECRET", "ssm:/ipp/dev/x")
    assert resolve_secret("TEST_SECRET", _FakeSecretProvider("resolved-value")) == "resolved-value"


def test_resolve_secret_ssm_without_provider(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TEST_SECRET", "ssm:/ipp/dev/x")
    with pytest.raises(RuntimeError):
        resolve_secret("TEST_SECRET", None)
