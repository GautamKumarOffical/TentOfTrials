use tent_backend::protocol::serialize::{Serializer, EncodingFormat, SchemaValidator};
use tent_backend::protocol::validate::{
    MessageValidator, validate_email, validate_uuid, validate_symbol,
    validate_instrument_id, validate_price, validate_quantity,
};
use tent_backend::protocol::codec::{FrameEncoder, FrameDecoder, Frame, FRAME_MAGIC, FRAME_HEADER_SIZE, FLAG_CHECKSUMED};
use tent_backend::protocol::{ProtocolError, MAX_MESSAGE_SIZE};
use tent_backend::protocol::messages::{MessageEnvelope, MessageRegistry, ids};
use serde_json::json;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn serializer() -> Serializer {
    Serializer::new(EncodingFormat::Json)
}

// ---------------------------------------------------------------------------
// Malformed JSON Deserialization
// ---------------------------------------------------------------------------

#[test]
fn deserialize_empty_bytes_returns_error() {
    let s = serializer();
    let result = s.deserialize::<serde_json::Value>(&[]);
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn deserialize_invalid_json_returns_error() {
    let s = serializer();
    let result = s.deserialize::<serde_json::Value>(b"not valid json {{{");
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn deserialize_truncated_json_returns_error() {
    let s = serializer();
    let result = s.deserialize::<serde_json::Value>(b"{\"key\": \"val");
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn deserialize_wrong_type_returns_error() {
    let s = serializer();
    let result = s.deserialize::<MessageEnvelope>(b"{\"not\": \"an envelope\"}");
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn deserialize_numeric_garbage_returns_error() {
    let s = serializer();
    let result = s.deserialize::<MessageEnvelope>(b"12345678901234567890");
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn deserialize_oversized_payload_returns_error() {
    let s = serializer();
    let oversized = vec![b'x'; MAX_MESSAGE_SIZE + 1];
    let result = s.deserialize::<serde_json::Value>(&oversized);
    assert!(matches!(result, Err(ProtocolError::MessageTooLarge)));
}

// ---------------------------------------------------------------------------
// Missing Required Fields
// ---------------------------------------------------------------------------

#[test]
fn validate_order_missing_side() {
    let payload = json!({
        "type": "limit",
        "quantity": 100,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "side"));
}

#[test]
fn validate_order_missing_type() {
    let payload = json!({
        "side": "buy",
        "quantity": 100,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "type"));
}

#[test]
fn validate_order_missing_quantity() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "quantity"));
}

#[test]
fn validate_order_missing_price_for_limit() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "quantity": 100
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "price"));
}

#[test]
fn validate_order_empty_payload() {
    let payload = json!({});
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.len() >= 3);
}

// ---------------------------------------------------------------------------
// Invalid Values
// ---------------------------------------------------------------------------

#[test]
fn validate_order_invalid_side() {
    let payload = json!({
        "side": "up",
        "type": "limit",
        "quantity": 100,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "side"));
}

#[test]
fn validate_order_invalid_type() {
    let payload = json!({
        "side": "buy",
        "type": "turbo",
        "quantity": 100,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "type"));
}

#[test]
fn validate_order_negative_quantity() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "quantity": -10,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "quantity"));
}

#[test]
fn validate_order_zero_quantity() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "quantity": 0,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
}

#[test]
fn validate_order_excessive_quantity() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "quantity": 2000000.0,
        "price": 50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "quantity"));
}

#[test]
fn validate_order_negative_price() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "quantity": 100,
        "price": -50.0
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "price"));
}

#[test]
fn validate_order_invalid_time_in_force() {
    let payload = json!({
        "side": "buy",
        "type": "limit",
        "quantity": 100,
        "price": 50.0,
        "time_in_force": "forever"
    });
    let result = MessageValidator::validate_order_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "time_in_force"));
}

#[test]
fn validate_account_invalid_currency() {
    let payload = json!({
        "amount": 100.0,
        "currency": "XYZ"
    });
    let result = MessageValidator::validate_account_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "currency"));
}

#[test]
fn validate_account_negative_amount() {
    let payload = json!({
        "amount": -100.0,
        "currency": "USD"
    });
    let result = MessageValidator::validate_account_payload(&payload);
    assert!(!result.valid);
    assert!(result.errors.iter().any(|e| e.field == "amount"));
}

// ---------------------------------------------------------------------------
// Schema Validation
// ---------------------------------------------------------------------------

#[test]
fn schema_validator_rejects_unknown_schema() {
    let v = SchemaValidator::new();
    let result = v.validate(0x9999, 1, b"{}");
    assert!(matches!(result, Err(ProtocolError::SchemaMismatch)));
}

#[test]
fn schema_validator_rejects_invalid_json_payload() {
    let mut v = SchemaValidator::new();
    v.register_schema(0x0001, 1, r#"{"properties": {}}"#).unwrap();
    let result = v.validate(0x0001, 1, b"not json");
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn schema_validator_rejects_non_object_payload() {
    let mut v = SchemaValidator::new();
    v.register_schema(0x0001, 1, r#"{"properties": {}}"#).unwrap();
    let result = v.validate(0x0001, 1, b"\"just a string\"");
    assert!(matches!(result, Err(ProtocolError::ValidationFailed)));
}

#[test]
fn schema_validator_rejects_missing_required_field() {
    let mut v = SchemaValidator::new();
    let schema = r#"{
        "properties": {
            "name": {"type": "string"},
            "age": {"type": "number"}
        }
    }"#;
    v.register_schema(0x0001, 1, schema).unwrap();
    let result = v.validate(0x0001, 1, br#"{"name": "Alice"}"#);
    assert!(matches!(result, Err(ProtocolError::ValidationFailed)));
}

#[test]
fn schema_validator_rejects_invalid_regex_pattern() {
    let mut v = SchemaValidator::new();
    let schema = r#"{
        "properties": {
            "email": {"type": "string", "pattern": "[invalid"}
        }
    }"#;
    v.register_schema(0x0001, 1, schema).unwrap();
    let result = v.validate(0x0001, 1, br#"{"email": "test@example.com"}"#);
    assert!(matches!(result, Err(ProtocolError::ValidationFailed)));
}

#[test]
fn schema_validator_rejects_invalid_schema_json() {
    let mut v = SchemaValidator::new();
    let result = v.register_schema(0x0001, 1, "not json");
    assert!(result.is_err());
}

#[test]
fn schema_validator_rejects_non_object_schema() {
    let mut v = SchemaValidator::new();
    let result = v.register_schema(0x0001, 1, "\"just a string\"");
    assert!(result.is_err());
}

// ---------------------------------------------------------------------------
// Frame Codec - Malformed Frames
// ---------------------------------------------------------------------------

#[test]
fn frame_decoder_rejects_wrong_magic() {
    let mut decoder = FrameDecoder::new();
    // Manually craft a buffer with wrong magic
    let mut bad_magic = Vec::new();
    bad_magic.extend_from_slice(&[0xDE, 0xAD, 0xBE, 0xEF]); // wrong magic
    bad_magic.push(3); // version
    bad_magic.push(1); // message_type
    bad_magic.extend_from_slice(&0u16.to_be_bytes()); // flags
    bad_magic.extend_from_slice(&5u32.to_be_bytes()); // payload_length
    bad_magic.extend_from_slice(&0u32.to_be_bytes()); // sequence
    bad_magic.extend_from_slice(&[0u8; 4]); // reserved
    bad_magic.extend_from_slice(b"hello"); // payload
    decoder.feed(&bad_magic);
    let result = decoder.decode();
    assert!(matches!(result, Err(ProtocolError::InvalidMessage)));
}

#[test]
fn frame_decoder_rejects_unsupported_version() {
    let frame = Frame::new(0x01, b"test".to_vec());
    let mut encoded = FrameEncoder::encode(&frame).unwrap();
    // Corrupt the version byte (byte 4 in the frame)
    encoded[4] = 99;
    let mut decoder = FrameDecoder::new();
    decoder.feed(&encoded);
    let result = decoder.decode();
    assert!(matches!(result, Err(ProtocolError::UnsupportedVersion)));
}

#[test]
fn frame_decoder_returns_none_for_incomplete_header() {
    let mut decoder = FrameDecoder::new();
    decoder.feed(&[0u8; 10]);
    let result = decoder.decode();
    assert!(matches!(result, Ok(None)));
}

#[test]
fn frame_encoder_rejects_oversized_payload() {
    let payload = vec![0u8; 16 * 1024 * 1024 + 1];
    let frame = Frame::new(0x01, payload);
    let result = FrameEncoder::encode(&frame);
    assert!(matches!(result, Err(ProtocolError::MessageTooLarge)));
}

#[test]
fn frame_validation_rejects_invalid_version() {
    let mut frame = Frame::new(0x01, b"test".to_vec());
    frame.version = 0; // below MIN_COMPATIBLE_VERSION
    assert!(!frame.is_valid());
}

#[test]
fn frame_validation_rejects_oversized_payload() {
    let mut frame = Frame::new(0x01, vec![0u8; 100]);
    // Manually set a payload that exceeds the limit
    frame.payload = vec![0u8; 16 * 1024 * 1024 + 1];
    assert!(!frame.is_valid());
}

#[test]
fn frame_validation_accepts_valid_frame() {
    let frame = Frame::new(0x01, b"test".to_vec());
    assert!(frame.is_valid());
}

// ---------------------------------------------------------------------------
// Convenience Validator Functions
// ---------------------------------------------------------------------------

#[test]
fn test_validate_email_valid() {
    assert!(validate_email("user@example.com"));
    assert!(validate_email("test.name+tag@domain.co"));
}

#[test]
fn test_validate_email_invalid() {
    assert!(!validate_email(""));
    assert!(!validate_email("not-an-email"));
    assert!(!validate_email("@domain.com"));
    assert!(!validate_email("user@"));
}

#[test]
fn test_validate_uuid_valid() {
    assert!(validate_uuid("550e8400-e29b-41d4-a716-446655440000"));
}

#[test]
fn test_validate_uuid_invalid() {
    assert!(!validate_uuid(""));
    assert!(!validate_uuid("not-a-uuid"));
    assert!(!validate_uuid("550e8400-e29b-41d4-a716"));
}

#[test]
fn test_validate_symbol_valid() {
    assert!(validate_symbol("BTC/USD"));
    assert!(validate_symbol("ETH/EUR"));
    assert!(validate_symbol("AAPL/USD"));
}

#[test]
fn test_validate_symbol_invalid() {
    assert!(!validate_symbol(""));
    assert!(!validate_symbol("BTC"));
    assert!(!validate_symbol("btc/usd"));
    assert!(!validate_symbol("TOOLONGSYMBOL/USD"));
}

#[test]
fn test_validate_instrument_id_valid() {
    assert!(validate_instrument_id("btcusd"));
    assert!(validate_instrument_id("eth01"));
}

#[test]
fn test_validate_instrument_id_invalid() {
    assert!(!validate_instrument_id(""));
    assert!(!validate_instrument_id("a"));
    assert!(!validate_instrument_id("BTCUSD"));
    assert!(!validate_instrument_id("has space"));
}

#[test]
fn test_validate_price_valid() {
    assert!(validate_price(0.01));
    assert!(validate_price(50000.0));
}

#[test]
fn test_validate_price_invalid() {
    assert!(!validate_price(0.0));
    assert!(!validate_price(-1.0));
    assert!(!validate_price(2_000_000_000.0));
}

#[test]
fn test_validate_quantity_valid() {
    assert!(validate_quantity(0.001));
    assert!(validate_quantity(100.0));
}

#[test]
fn test_validate_quantity_invalid() {
    assert!(!validate_quantity(0.0));
    assert!(!validate_quantity(-1.0));
    assert!(!validate_quantity(200_000_000.0));
}

// ---------------------------------------------------------------------------
// Serialization Edge Cases
// ---------------------------------------------------------------------------

#[test]
fn serialize_empty_object_succeeds() {
    let s = serializer();
    let result = s.serialize(&serde_json::json!({}));
    assert!(result.is_ok());
}

#[test]
fn serialize_nested_deep_structure_succeeds() {
    let s = serializer();
    let deep = serde_json::json!({
        "a": {"b": {"c": {"d": {"e": {"f": "deep"}}}}}
    });
    let result = s.serialize(&deep);
    assert!(result.is_ok());
}

#[test]
fn serialize_unicode_payload_succeeds() {
    let s = serializer();
    let unicode = serde_json::json!({
        "text": "Hello \u{1F600} World \u{00E9}\u{00E8}\u{00EA}"
    });
    let result = s.serialize(&unicode);
    assert!(result.is_ok());
}

#[test]
fn roundtrip_serialize_deserialize() {
    let s = serializer();
    let original = serde_json::json!({
        "key": "value",
        "number": 42,
        "nested": {"inner": true}
    });
    let bytes = s.serialize(&original).unwrap();
    let deserialized: serde_json::Value = s.deserialize(&bytes).unwrap();
    assert_eq!(original, deserialized);
}

// ---------------------------------------------------------------------------
// Message Envelope - Malformed Payloads
// ---------------------------------------------------------------------------

#[test]
fn deserialize_malformed_envelope_missing_fields() {
    let s = serializer();
    let result = s.deserialize::<MessageEnvelope>(br#"{"message_type": 1}"#);
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

#[test]
fn deserialize_envelope_wrong_field_types() {
    let s = serializer();
    let result = s.deserialize::<MessageEnvelope>(br#"{
        "message_id": "not_a_number",
        "message_type": "not_a_number",
        "schema_version": "not_a_number",
        "timestamp": "not_a_number",
        "priority": "not_a_number",
        "flags": "not_a_number",
        "payload": "not_bytes"
    }"#);
    assert!(matches!(result, Err(ProtocolError::DeserializationFailed)));
}

// ---------------------------------------------------------------------------
// Protocol Error Display
// ---------------------------------------------------------------------------

#[test]
fn protocol_error_display_messages_are_meaningful() {
    let errors = [
        ProtocolError::Unknown,
        ProtocolError::InvalidMessage,
        ProtocolError::UnsupportedVersion,
        ProtocolError::DeserializationFailed,
        ProtocolError::SerializationFailed,
        ProtocolError::ValidationFailed,
        ProtocolError::SchemaMismatch,
        ProtocolError::MessageTooLarge,
        ProtocolError::Timeout,
        ProtocolError::NotSupported,
        ProtocolError::InternalError,
        ProtocolError::ChecksumMismatch,
    ];
    for error in &errors {
        let msg = format!("{}", error);
        assert!(!msg.is_empty(), "error display should not be empty");
    }
}

// ---------------------------------------------------------------------------
// Message Registry - No Handler
// ---------------------------------------------------------------------------

#[test]
fn message_registry_returns_error_for_unregistered_type() {
    let registry = MessageRegistry::new();
    let result = registry.handle(0xFFFF, b"test");
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("No handler registered"));
}

// ---------------------------------------------------------------------------
// Version Negotiation
// ---------------------------------------------------------------------------

#[test]
fn version_negotiation_clamps_to_compatible_range() {
    use tent_backend::protocol::{VersionNegotiation, PROTOCOL_VERSION, MIN_COMPATIBLE_VERSION};

    // Client version below minimum
    let neg = VersionNegotiation::new(0, PROTOCOL_VERSION, 0);
    assert!(neg.negotiated_version >= MIN_COMPATIBLE_VERSION);

    // Client version above server
    let neg = VersionNegotiation::new(999, PROTOCOL_VERSION, 0);
    assert!(neg.negotiated_version <= PROTOCOL_VERSION);

    // Equal versions
    let neg = VersionNegotiation::new(PROTOCOL_VERSION, PROTOCOL_VERSION, 0);
    assert_eq!(neg.negotiated_version, PROTOCOL_VERSION);
}

#[test]
fn version_supported_check() {
    use tent_backend::protocol::{is_version_supported, PROTOCOL_VERSION, MIN_COMPATIBLE_VERSION};

    assert!(is_version_supported(MIN_COMPATIBLE_VERSION));
    assert!(is_version_supported(PROTOCOL_VERSION));
    assert!(!is_version_supported(0));
    assert!(!is_version_supported(PROTOCOL_VERSION + 1));
}

// ---------------------------------------------------------------------------
// Validation Result API
// ---------------------------------------------------------------------------

#[test]
fn validation_result_combine_preserves_errors() {
    use tent_backend::protocol::validate::ValidationResult;

    let mut a = ValidationResult::error("field1", "code1", "error1");
    let b = ValidationResult::error("field2", "code2", "error2");
    a.combine(b);

    assert!(!a.valid);
    assert_eq!(a.errors.len(), 2);
}

#[test]
fn validation_result_add_warning() {
    use tent_backend::protocol::validate::ValidationResult;

    let mut result = ValidationResult::valid();
    result.add_warning("something minor");
    assert!(result.valid);
    assert!(result.has_warnings());
    assert!(!result.has_errors());
}
