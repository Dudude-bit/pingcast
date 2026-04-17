"use client";

import { useFormContext, Controller } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { MonitorTypeInfo } from "@/lib/queries";

/**
 * Renders the dynamic per-type config fields under the `check_config.*`
 * form namespace. The schema comes from the Go /api/monitor-types endpoint
 * (registered per monitor type: HTTP, TCP, DNS).
 */
export function ConfigFields({ typeInfo }: { typeInfo: MonitorTypeInfo | undefined }) {
  const { register, control, formState: { errors } } = useFormContext();

  if (!typeInfo?.schema?.fields?.length) {
    return (
      <p className="text-sm text-muted-foreground">
        Select a monitor type to configure checks.
      </p>
    );
  }

  return (
    <div className="space-y-4">
      {typeInfo.schema.fields.map((field) => {
        const name = `check_config.${field.name}`;
        const errMsg = (errors?.check_config as Record<string, { message?: string }> | undefined)?.[field.name ?? ""]?.message;

        if (field.type === "select") {
          return (
            <div key={field.name} className="space-y-2">
              <Label htmlFor={name}>{field.label}</Label>
              <Controller
                control={control}
                name={name}
                defaultValue={field.default ?? ""}
                rules={{ required: field.required }}
                render={({ field: ctl }) => (
                  <Select value={String(ctl.value ?? "")} onValueChange={ctl.onChange}>
                    <SelectTrigger id={name}>
                      <SelectValue placeholder={field.placeholder} />
                    </SelectTrigger>
                    <SelectContent>
                      {field.options?.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value ?? ""}>
                          {opt.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {errMsg ? <p className="text-xs text-red-600">{errMsg}</p> : null}
            </div>
          );
        }

        const inputType = field.type === "number" ? "number" : "text";
        return (
          <div key={field.name} className="space-y-2">
            <Label htmlFor={name}>{field.label}</Label>
            <Input
              id={name}
              type={inputType}
              placeholder={field.placeholder}
              defaultValue={field.default === undefined ? undefined : String(field.default)}
              {...register(name, {
                required: field.required,
                valueAsNumber: inputType === "number",
              })}
            />
            {errMsg ? <p className="text-xs text-red-600">{errMsg}</p> : null}
          </div>
        );
      })}
    </div>
  );
}
