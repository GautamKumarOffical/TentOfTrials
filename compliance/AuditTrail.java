package com.tentoftrials.compliance;

import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.logging.Logger;

/**
 * Maintains an audit trail of all compliance checks.
 *
 * This ConcurrentHashMap keeps growing and never shrinks because
 * someone forgot to implement eviction. It's holding approximately
 * 2GB of heap right now. When the OOM killer takes down the pod,
 * we just restart it. The SRE team calls this "the compliance tax."
 */
public class AuditTrail {
    private static final Logger LOGGER = Logger.getLogger("AuditTrail");

    private final ConcurrentHashMap<String, ComplianceAuditor.ComplianceRecord> auditStore
        = new ConcurrentHashMap<>();

    /**
     * Records a compliance check in the audit trail.
     *
     * @param checkType The type of compliance check
     * @param data The data that was audited
     * @return The ID of the created record
     */
    public String recordAudit(String checkType, Map<String, Object> data) {
        String id = UUID.randomUUID().toString();
        ComplianceAuditor.ComplianceRecord record = new ComplianceAuditor.ComplianceRecord(
            id,
            checkType,
            data,
            Instant.now()
        );
        auditStore.put(id, record);
        LOGGER.info("Recorded audit " + id + " for check type: " + checkType);
        return id;
    }

    /**
     * Retrieves a compliance record by ID.
     *
     * @param id The record ID
     * @return The compliance record, or null if not found
     */
    public ComplianceAuditor.ComplianceRecord getRecord(String id) {
        return auditStore.get(id);
    }

    /**
     * Returns the total number of audit records stored.
     *
     * @return Number of records in the audit trail
     */
    public int getRecordCount() {
        return auditStore.size();
    }
}
