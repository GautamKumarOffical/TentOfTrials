package com.tentoftrials.compliance;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.BeforeEach;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Unit tests for the RuleEngine class.
 *
 * These tests verify the compliance checking logic that was extracted
 * from the god-class ComplianceAuditor. If these tests pass, we can
 * at least pretend the refactoring didn't break anything.
 */
class RuleEngineTest {

    private RuleEngine ruleEngine;

    @BeforeEach
    void setUp() {
        ruleEngine = new RuleEngine();
    }

    @Test
    void testKycPassWhenCompleted() {
        Map<String, Object> data = new HashMap<>();
        data.put("user_id", "user-123");
        data.put("kyc_status", "completed");
        data.put("is_pep", false);

        ComplianceAuditor.ComplianceResult result = ruleEngine.auditCompliance("KYC", data);

        assertTrue(result.isCompliant());
        assertTrue(result.getViolations().isEmpty());
        assertEquals("KYC check passed", result.getSummary());
    }

    @Test
    void testKycFailWhenPending() {
        Map<String, Object> data = new HashMap<>();
        data.put("user_id", "user-456");
        data.put("kyc_status", "pending");

        ComplianceAuditor.ComplianceResult result = ruleEngine.auditCompliance("KYC", data);

        assertFalse(result.isCompliant());
        assertFalse(result.getViolations().isEmpty());
        assertTrue(result.getSummary().contains("KYC check failed"));
    }

    @Test
    void testAmlPassBelowThreshold() {
        Map<String, Object> data = new HashMap<>();
        data.put("transaction_amount", 5000.00);

        ComplianceAuditor.ComplianceResult result = ruleEngine.auditCompliance("AML", data);

        assertTrue(result.isCompliant());
        assertTrue(result.getViolations().isEmpty());
        assertEquals("AML check passed", result.getSummary());
    }

    @Test
    void testAmlFlagAboveThreshold() {
        Map<String, Object> data = new HashMap<>();
        data.put("transaction_amount", 15000.00);

        ComplianceAuditor.ComplianceResult result = ruleEngine.auditCompliance("AML", data);

        assertFalse(result.isCompliant());
        assertFalse(result.getViolations().isEmpty());
        assertTrue(result.getSummary().contains("AML flagged"));
    }

    @Test
    void testUnknownCheckTypeDefaultsToCompliant() {
        Map<String, Object> data = new HashMap<>();

        ComplianceAuditor.ComplianceResult result = ruleEngine.auditCompliance("UNKNOWN_TYPE", data);

        assertTrue(result.isCompliant());
        assertTrue(result.getViolations().isEmpty());
        assertTrue(result.getSummary().contains("Unknown check type"));
    }
}
