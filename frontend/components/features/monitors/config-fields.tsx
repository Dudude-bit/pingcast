"use client";

import { DynamicConfigFields } from "@/components/features/common/dynamic-config-fields";
import type { MonitorTypeInfo } from "@/lib/queries";

/**
 * Thin wrapper around DynamicConfigFields that pulls fields from
 * MonitorTypeInfo.schema.fields. Form values live under `check_config.*`.
 */
export function ConfigFields({
  typeInfo,
}: {
  typeInfo: MonitorTypeInfo | undefined;
}) {
  return (
    <DynamicConfigFields
      fields={typeInfo?.schema?.fields}
      namespace="check_config"
    />
  );
}
