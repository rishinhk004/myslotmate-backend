-- Migration: Make payout_methods.host_id nullable for platform-level payout methods
-- Purpose: Allow platform (admin) accounts to have payout methods without a host_id
-- Background: Platform withdrawals need payout methods but don't belong to a specific host

-- Drop the existing foreign key constraint
ALTER TABLE payout_methods 
DROP CONSTRAINT payout_methods_host_id_fkey;

-- Make host_id nullable
ALTER TABLE payout_methods 
ALTER COLUMN host_id DROP NOT NULL;

-- Re-add the foreign key constraint (now allowing NULL for platform methods)
ALTER TABLE payout_methods 
ADD CONSTRAINT payout_methods_host_id_fkey 
FOREIGN KEY (host_id) REFERENCES hosts (id) ON DELETE CASCADE;

-- Comment on the change
COMMENT ON COLUMN payout_methods.host_id IS 
  'Host ID for host payout methods. NULL for platform-level admin payout methods.';
