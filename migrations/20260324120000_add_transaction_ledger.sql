-- Transaction Ledger: Immutable journal of all wallet movements (Swiggy/Zomato style)
-- Balance is calculated as SUM of ledger entries, preventing tampering

CREATE TABLE transaction_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Who owns this ledger entry
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    
    -- Transaction type
    type VARCHAR(50) NOT NULL,
    
    -- Amount in smallest currency unit (cents)
    -- POSITIVE = credit (money in)
    -- NEGATIVE = debit (money out)
    amount_cents INT8 NOT NULL,
    
    -- Reference to source transaction (for reconciliation)
    reference_id UUID,
    reference_type VARCHAR(50),
    
    -- Idempotency key to prevent duplicate ledger entries
    idempotency_key VARCHAR(255) UNIQUE,
    
    -- Description for audit
    description TEXT,
    
    -- Current balance AFTER this entry (cached for performance, verified by reconciliation)
    balance_after_cents INT8 NOT NULL,
    
    -- Webhook/External reference for audit trail
    external_reference_id VARCHAR(255),
    
    -- For reversals: points to original ledger entry being reversed
    reversal_of_ledger_id UUID REFERENCES transaction_ledger(id),
    
    -- Status tracking for async operations
    status VARCHAR(50) NOT NULL DEFAULT 'completed',
    
    -- Metadata for complex scenarios
    metadata JSONB,
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID
);

-- Create indexes for transaction_ledger
CREATE INDEX idx_ledger_account_id ON transaction_ledger(account_id);
CREATE INDEX idx_ledger_reference ON transaction_ledger(reference_id, reference_type);
CREATE INDEX idx_ledger_created_at ON transaction_ledger(created_at DESC);
CREATE INDEX idx_ledger_idempotency ON transaction_ledger(idempotency_key);
CREATE INDEX idx_ledger_status ON transaction_ledger(status);

-- Table for tracking webhooks processed (prevent replay attacks)
CREATE TABLE webhook_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Which webhook event
    event_type VARCHAR(50) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    external_event_id VARCHAR(255) NOT NULL,
    
    -- Idempotency scope
    idempotency_key VARCHAR(255) NOT NULL,
    
    -- Result of processing
    ledger_id UUID REFERENCES transaction_ledger(id),
    status VARCHAR(50) NOT NULL,
    error_message TEXT,
    
    -- Raw webhook payload (for debugging)
    raw_payload JSONB NOT NULL,
    
    -- Timestamps
    received_at TIMESTAMP NOT NULL,
    processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_webhook_event UNIQUE(event_type, provider_id, external_event_id)
);

-- Create indexes for webhook_executions
CREATE INDEX idx_webhook_event_type ON webhook_executions(event_type);
CREATE INDEX idx_webhook_processed ON webhook_executions(processed_at DESC);

-- Audit log for sensitive operations (account changes, payout method changes, balance discrepancies)
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    
    action VARCHAR(50) NOT NULL,
    
    -- Who did it
    actor_id UUID,
    actor_type VARCHAR(50),
    
    -- What changed
    old_values JSONB,
    new_values JSONB,
    
    ip_address VARCHAR(50),
    user_agent TEXT,
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for audit_log
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_created_at ON audit_log(created_at DESC);

-- Reconciliation results (daily check that ledger sum = account balance)
CREATE TABLE reconciliation_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    run_date DATE NOT NULL,
    
    -- Global stats
    total_accounts_checked INT NOT NULL,
    accounts_matched INT NOT NULL,
    accounts_discrepancy INT NOT NULL,
    
    -- Amounts in cents
    total_ledger_amount_cents INT8 NOT NULL,
    total_balance_amount_cents INT8 NOT NULL,
    
    status VARCHAR(50) NOT NULL,
    error_details JSONB,
    
    run_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

-- Create index for reconciliation_runs
CREATE INDEX idx_reconciliation_run_date ON reconciliation_runs(run_date DESC);

-- Discrepancies found during reconciliation (for investigation)
CREATE TABLE reconciliation_discrepancies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    reconciliation_run_id UUID NOT NULL REFERENCES reconciliation_runs(id) ON DELETE CASCADE,
    
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    
    -- The mismatch
    stored_balance_cents INT8 NOT NULL,
    ledger_calculated_cents INT8 NOT NULL,
    difference_cents INT8 NOT NULL,
    
    -- Analysis
    reason TEXT,
    severity VARCHAR(50),
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for reconciliation_discrepancies
CREATE INDEX idx_discrepancy_run ON reconciliation_discrepancies(reconciliation_run_id);
CREATE INDEX idx_discrepancy_account ON reconciliation_discrepancies(account_id);

COMMENT ON TABLE transaction_ledger IS 'Immutable transaction journal. Balance = SUM(amount_cents) for account_id. Prevents DB tampering.';
COMMENT ON TABLE webhook_executions IS 'Track webhook processing to prevent replay attacks and detect duplicates.';
COMMENT ON TABLE audit_log IS 'Audit trail for all sensitive operations on accounts, payouts, etc.';
COMMENT ON TABLE reconciliation_runs IS 'Daily reconciliation results. Stored balance must equal SUM(ledger).';
