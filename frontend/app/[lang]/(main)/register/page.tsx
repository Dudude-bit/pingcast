"use client";

import Link from "next/link";
import { useActionState } from "react";
import { register } from "@/app/actions/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useLocale } from "@/components/i18n/locale-provider";

export default function RegisterPage() {
  const { dict, locale } = useLocale();
  const a = dict.auth;
  const [state, formAction, pending] = useActionState(register, {});

  return (
    <div className="container mx-auto px-4 py-16 max-w-md">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{a.register_title}</CardTitle>
          <CardDescription>{a.register_sub}</CardDescription>
        </CardHeader>
        <CardContent>
          {state.error ? (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>{state.error}</AlertDescription>
            </Alert>
          ) : null}

          <form action={formAction} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">{a.register_email}</Label>
              <Input
                id="email"
                name="email"
                type="email"
                required
                autoComplete="email"
                autoFocus
                placeholder="you@company.com"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="slug">{a.register_slug}</Label>
              <Input
                id="slug"
                name="slug"
                type="text"
                required
                pattern="[a-z0-9-]{3,30}"
                placeholder="your-company"
              />
              <p className="text-xs text-muted-foreground">
                {a.register_slug_help}
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">{a.register_password}</Label>
              <Input
                id="password"
                name="password"
                type="password"
                required
                minLength={8}
                autoComplete="new-password"
              />
              <p className="text-xs text-muted-foreground">
                {a.register_password_help}
              </p>
            </div>
            <Button type="submit" className="w-full" disabled={pending}>
              {pending ? `${dict.common.loading}` : a.register_submit}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-muted-foreground">
            {a.register_have_account}{" "}
            <Link
              href={`/${locale}/login`}
              className="underline underline-offset-4 hover:text-foreground"
            >
              {a.register_login_link}
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
