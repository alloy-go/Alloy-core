import { SignupForm } from '@/components/auth/signup-form';

export default function SignupPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md space-y-6">
        <div className="flex flex-col items-center space-y-2">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 bg-primary rounded-lg flex items-center justify-center">
              <span className="text-primary-foreground font-bold text-xl">M</span>
            </div>
            <div>
              <h1 className="text-3xl font-bold tracking-tight">Minimon CD</h1>
              <p className="text-sm text-muted-foreground">v1.0.0</p>
            </div>
          </div>
        </div>
        <SignupForm />
      </div>
    </div>
  );
}
