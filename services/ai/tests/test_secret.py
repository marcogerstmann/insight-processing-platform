from ipp_ai.domain.secret import SecretRef, parse_secret_ref


def test_parse_secret_ref_literal() -> None:
    assert parse_secret_ref("sk-proj-abc123") == SecretRef(
        value="sk-proj-abc123", is_ssm_path=False
    )


def test_parse_secret_ref_ssm_path() -> None:
    assert parse_secret_ref("ssm:/ipp/dev/openai/api_key") == SecretRef(
        value="/ipp/dev/openai/api_key", is_ssm_path=True
    )
