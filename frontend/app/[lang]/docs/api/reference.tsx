"use client";

import { ApiReferenceReact } from "@scalar/api-reference-react";
import "@scalar/api-reference-react/style.css";

export function ApiReference() {
  return (
    <div className="min-h-screen bg-background">
      <ApiReferenceReact
        configuration={{
          url: "/openapi.yaml",
          theme: "default",
          hideClientButton: false,
          layout: "modern",
        }}
      />
    </div>
  );
}
