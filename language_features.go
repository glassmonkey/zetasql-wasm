package zetasql

import "github.com/glassmonkey/zetasql-wasm/wasm/generated"

// LanguageFeature constants. These are the most commonly enabled features
// when configuring the analyzer; for the full list of values supported by
// the underlying proto, see generated.LanguageFeature.

// Plain (non-versioned) features.
const (
	FeatureAnalyticFunctions             LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_ANALYTIC_FUNCTIONS)
	FeatureBignumericType                LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_BIGNUMERIC_TYPE)
	FeatureCreateTableAsSelectColumnList LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_CREATE_TABLE_AS_SELECT_COLUMN_LIST)
	FeatureCreateTableNotNull            LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_CREATE_TABLE_NOT_NULL)
	FeatureGeography                     LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_GEOGRAPHY)
	FeatureGroupByRollup                 LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_GROUP_BY_ROLLUP)
	FeatureIntervalType                  LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_INTERVAL_TYPE)
	FeatureJsonArrayFunctions            LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_JSON_ARRAY_FUNCTIONS)
	FeatureJsonStrictNumberParsing       LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_JSON_STRICT_NUMBER_PARSING)
	FeatureJsonType                      LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_JSON_TYPE)
	FeatureNamedArguments                LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_NAMED_ARGUMENTS)
	FeatureNumericType                   LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_NUMERIC_TYPE)
	FeatureParameterizedTypes            LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_PARAMETERIZED_TYPES)
	FeatureTablesample                   LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_TABLESAMPLE)
	FeatureTemplateFunctions             LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_TEMPLATE_FUNCTIONS)
	FeatureTimestampNanos                LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_TIMESTAMP_NANOS)
)

// V_1_1 features.
const (
	FeatureV11HavingInAggregate               LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_HAVING_IN_AGGREGATE)
	FeatureV11LimitInAggregate                LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_LIMIT_IN_AGGREGATE)
	FeatureV11NullHandlingModifierInAggregate LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_NULL_HANDLING_MODIFIER_IN_AGGREGATE)
	FeatureV11NullHandlingModifierInAnalytic  LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_NULL_HANDLING_MODIFIER_IN_ANALYTIC)
	FeatureV11OrderByCollate                  LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_ORDER_BY_COLLATE)
	FeatureV11OrderByInAggregate              LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_ORDER_BY_IN_AGGREGATE)
	FeatureV11SelectStarExceptReplace         LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_SELECT_STAR_EXCEPT_REPLACE)
	FeatureV11WithOnSubquery                  LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_1_WITH_ON_SUBQUERY)
)

// V_1_2 features.
const (
	FeatureV12CivilTime        LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_2_CIVIL_TIME)
	FeatureV12SafeFunctionCall LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_2_SAFE_FUNCTION_CALL)
	FeatureV12WeekWithWeekday  LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_2_WEEK_WITH_WEEKDAY)
)

// V_1_3 features.
const (
	FeatureV13AllowDashesInTableName     LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_ALLOW_DASHES_IN_TABLE_NAME)
	FeatureV13DateArithmetics            LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_DATE_ARITHMETICS)
	FeatureV13DateTimeConstructors       LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_DATE_TIME_CONSTRUCTORS)
	FeatureV13DecimalAlias               LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_DECIMAL_ALIAS)
	FeatureV13ExtendedDateTimeSignatures LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_EXTENDED_DATE_TIME_SIGNATURES)
	FeatureV13ExtendedGeographyParsers   LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_EXTENDED_GEOGRAPHY_PARSERS)
	FeatureV13FormatInCast               LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_FORMAT_IN_CAST)
	FeatureV13IsDistinct                 LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_IS_DISTINCT)
	FeatureV13NullsFirstLastInOrderBy    LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_NULLS_FIRST_LAST_IN_ORDER_BY)
	FeatureV13Pivot                      LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_PIVOT)
	FeatureV13Qualify                    LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_QUALIFY)
	FeatureV13Unpivot                    LanguageFeature = LanguageFeature(generated.LanguageFeature_FEATURE_V_1_3_UNPIVOT)
)
