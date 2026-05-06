package zetasql

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// StatementKind identifies a top-level SQL statement form (QUERY, INSERT,
// CREATE TABLE, …) for use with LanguageOptions.SetSupportedStatementKinds.
//
// The underlying proto enum (generated.ResolvedNodeKind) carries every
// resolved-AST node kind, including hundreds of non-statement values.
// StatementKind is a named subset wrapper: the constants below cover the
// statement (RESOLVED_*_STMT) values, so callers configuring an analyzer
// don't need to import wasm/generated. Wire identity is preserved (both
// are int32) so values round-trip through toProto without conversion loss.
type StatementKind int32

// String returns the canonical proto enum name (e.g. "RESOLVED_QUERY_STMT").
func (k StatementKind) String() string {
	return generated.ResolvedNodeKind(k).String()
}

func (k StatementKind) toProto() generated.ResolvedNodeKind {
	return generated.ResolvedNodeKind(k)
}

// Statement kind constants. One per RESOLVED_*_STMT value in the underlying
// proto enum.
const (
	StatementKindAbortBatch                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ABORT_BATCH_STMT)
	StatementKindAlterAllRowAccessPolicies  StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_ALL_ROW_ACCESS_POLICIES_STMT)
	StatementKindAlterApproxView            StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_APPROX_VIEW_STMT)
	StatementKindAlterConnection            StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_CONNECTION_STMT)
	StatementKindAlterDatabase              StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_DATABASE_STMT)
	StatementKindAlterEntity                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_ENTITY_STMT)
	StatementKindAlterExternalSchema        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_EXTERNAL_SCHEMA_STMT)
	StatementKindAlterIndex                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_INDEX_STMT)
	StatementKindAlterMaterializedView      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_MATERIALIZED_VIEW_STMT)
	StatementKindAlterModel                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_MODEL_STMT)
	StatementKindAlterPrivilegeRestriction  StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_PRIVILEGE_RESTRICTION_STMT)
	StatementKindAlterRowAccessPolicy       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_ROW_ACCESS_POLICY_STMT)
	StatementKindAlterSchema                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_SCHEMA_STMT)
	StatementKindAlterSequence              StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_SEQUENCE_STMT)
	StatementKindAlterTableSetOptions       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_TABLE_SET_OPTIONS_STMT)
	StatementKindAlterTable                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_TABLE_STMT)
	StatementKindAlterView                  StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ALTER_VIEW_STMT)
	StatementKindAnalyze                    StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ANALYZE_STMT)
	StatementKindAssert                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ASSERT_STMT)
	StatementKindAssignment                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ASSIGNMENT_STMT)
	StatementKindAuxLoadData                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_AUX_LOAD_DATA_STMT)
	StatementKindBegin                      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_BEGIN_STMT)
	StatementKindCall                       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CALL_STMT)
	StatementKindCloneData                  StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CLONE_DATA_STMT)
	StatementKindCommit                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_COMMIT_STMT)
	StatementKindCreateApproxView           StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_APPROX_VIEW_STMT)
	StatementKindCreateConnection           StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_CONNECTION_STMT)
	StatementKindCreateConstant             StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_CONSTANT_STMT)
	StatementKindCreateDatabase             StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_DATABASE_STMT)
	StatementKindCreateEntity               StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_ENTITY_STMT)
	StatementKindCreateExternalSchema       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_EXTERNAL_SCHEMA_STMT)
	StatementKindCreateExternalTable        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_EXTERNAL_TABLE_STMT)
	StatementKindCreateFunction             StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_FUNCTION_STMT)
	StatementKindCreateIndex                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_INDEX_STMT)
	StatementKindCreateMaterializedView     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_MATERIALIZED_VIEW_STMT)
	StatementKindCreateModel                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_MODEL_STMT)
	StatementKindCreatePrivilegeRestriction StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_PRIVILEGE_RESTRICTION_STMT)
	StatementKindCreateProcedure            StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_PROCEDURE_STMT)
	StatementKindCreatePropertyGraph        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_PROPERTY_GRAPH_STMT)
	StatementKindCreateRowAccessPolicy      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_ROW_ACCESS_POLICY_STMT)
	StatementKindCreateSchema               StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_SCHEMA_STMT)
	StatementKindCreateSequence             StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_SEQUENCE_STMT)
	StatementKindCreateSnapshotTable        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_SNAPSHOT_TABLE_STMT)
	StatementKindCreateTableAsSelect        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_TABLE_AS_SELECT_STMT)
	StatementKindCreateTableFunction        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_TABLE_FUNCTION_STMT)
	StatementKindCreateTable                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_TABLE_STMT)
	StatementKindCreateView                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_VIEW_STMT)
	StatementKindCreateWithEntry            StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_CREATE_WITH_ENTRY_STMT)
	StatementKindDefineTable                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DEFINE_TABLE_STMT)
	StatementKindDelete                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DELETE_STMT)
	StatementKindDescribe                   StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DESCRIBE_STMT)
	StatementKindDropFunction               StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_FUNCTION_STMT)
	StatementKindDropIndex                  StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_INDEX_STMT)
	StatementKindDropMaterializedView       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_MATERIALIZED_VIEW_STMT)
	StatementKindDropPrivilegeRestriction   StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_PRIVILEGE_RESTRICTION_STMT)
	StatementKindDropRowAccessPolicy        StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_ROW_ACCESS_POLICY_STMT)
	StatementKindDropSnapshotTable          StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_SNAPSHOT_TABLE_STMT)
	StatementKindDrop                       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_STMT)
	StatementKindDropTableFunction          StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_DROP_TABLE_FUNCTION_STMT)
	StatementKindExecuteImmediate           StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_EXECUTE_IMMEDIATE_STMT)
	StatementKindExplain                    StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_EXPLAIN_STMT)
	StatementKindExportData                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_EXPORT_DATA_STMT)
	StatementKindExportMetadata             StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_EXPORT_METADATA_STMT)
	StatementKindExportModel                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_EXPORT_MODEL_STMT)
	StatementKindGeneralizedQuery           StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_GENERALIZED_QUERY_STMT)
	StatementKindGrant                      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_GRANT_STMT)
	StatementKindImport                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_IMPORT_STMT)
	StatementKindInsert                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_INSERT_STMT)
	StatementKindMerge                      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_MERGE_STMT)
	StatementKindModule                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_MODULE_STMT)
	StatementKindMulti                      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_MULTI_STMT)
	StatementKindQuery                      StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_QUERY_STMT)
	StatementKindRename                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_RENAME_STMT)
	StatementKindRevoke                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_REVOKE_STMT)
	StatementKindRollback                   StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_ROLLBACK_STMT)
	StatementKindRunBatch                   StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_RUN_BATCH_STMT)
	StatementKindSetTransaction             StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_SET_TRANSACTION_STMT)
	StatementKindShow                       StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_SHOW_STMT)
	StatementKindStartBatch                 StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_START_BATCH_STMT)
	StatementKindStatementWithPipeOperators StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_STATEMENT_WITH_PIPE_OPERATORS_STMT)
	StatementKindSubpipeline                StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_SUBPIPELINE_STMT)
	StatementKindTruncate                   StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_TRUNCATE_STMT)
	StatementKindUndrop                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_UNDROP_STMT)
	StatementKindUpdate                     StatementKind = StatementKind(generated.ResolvedNodeKind_RESOLVED_UPDATE_STMT)
)
