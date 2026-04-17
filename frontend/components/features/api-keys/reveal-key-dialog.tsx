"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Check, Copy, KeyRound } from "lucide-react";

interface Props {
  rawKey: string | null;
  onClose: () => void;
}

export function RevealKeyDialog({ rawKey, onClose }: Props) {
  const [copied, setCopied] = useState(false);

  const onCopy = async () => {
    if (!rawKey) return;
    await navigator.clipboard.writeText(rawKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog open={rawKey !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <div className="inline-flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary mb-2">
            <KeyRound className="h-5 w-5" />
          </div>
          <DialogTitle>Your new API key</DialogTitle>
          <DialogDescription>
            This is the only time you will see this key. Copy it now and store it somewhere safe.
          </DialogDescription>
        </DialogHeader>

        <Alert variant="destructive" className="my-2">
          <AlertDescription>
            If you lose it, you will need to revoke this key and create a new one.
          </AlertDescription>
        </Alert>

        <div className="rounded-md bg-muted/60 border border-border/60 p-3 font-mono text-sm break-all select-all">
          {rawKey}
        </div>

        <DialogFooter className="gap-2">
          <Button onClick={onCopy} variant={copied ? "secondary" : "default"}>
            {copied ? (
              <>
                <Check className="mr-2 h-4 w-4" /> Copied
              </>
            ) : (
              <>
                <Copy className="mr-2 h-4 w-4" /> Copy key
              </>
            )}
          </Button>
          <Button variant="ghost" onClick={onClose}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
