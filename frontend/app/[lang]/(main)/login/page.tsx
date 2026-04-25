"use client";

import Link from "next/link";
import { useActionState } from "react";
import { login } from "@/app/actions/auth";
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

export default function LoginPage() {
  const { dict, locale } = useLocale();
  const a = dict.auth;
  const [state, formAction, pending] = useActionState(login, {});

  return (
    <div className="container mx-auto px-4 py-16 max-w-md">
      <Card>
        <CardHeader>
          <CardTitle className="text-2xl">{a.login_title}</CardTitle>
          <CardDescription>{a.login_sub}</CardDescription>
        </CardHeader>
        <CardContent>
          {state.error ? (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>{state.error}</AlertDescription>
            </Alert>
          ) : null}

          <form action={formAction} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">{a.login_email}</Label>
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
              <Label htmlFor="password">{a.login_password}</Label>
              <Input
                id="password"
                name="password"
                type="password"
                required
                minLength={8}
                autoComplete="current-password"
              />
            </div>
            <Button type="submit" className="w-full" disabled={pending}>
              {pending ? `${dict.common.loading}` : a.login_submit}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-muted-foreground">
            {a.login_no_account}{" "}
            <Link
              href={`/${locale}/register`}
              className="underline underline-offset-4 hover:text-foreground"
            >
              {a.login_register_link}
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
