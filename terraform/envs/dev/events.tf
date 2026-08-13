# ---------------------------------------------------------------
# Domain events bus — separate from the ingest work queue (SQS).
# Domain events (InsightCreated, InsightEnriched, ...) go here so
# subscriber rules aren't reading pipeline transport traffic.
# ---------------------------------------------------------------

module "domain_events_bus" {
  source = "../../modules/eventbridge"
  name   = "${var.project}-${var.env}-domain-events"
}
