'use client';

import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import { Button } from '@/components/ui/button';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { projectAPI, APIError } from '@/lib/api';
import { Loader2 } from 'lucide-react';

const projectFormSchema = z.object({
  projectName: z
    .string()
    .min(3, 'Project name must be at least 3 characters')
    .max(50, 'Project name must not exceed 50 characters')
    .regex(/^[a-z0-9-]+$/, 'Project name must contain only lowercase letters, numbers, and hyphens'),
  deploymentType: z.enum(['deployment', 'statefulset', 'daemonset', 'job', 'cronjob']),
  contextName: z
    .string()
    .min(1, 'Context name is required')
    .max(100, 'Context name must not exceed 100 characters'),
});



type ProjectFormValues = z.infer<typeof projectFormSchema>;

interface NewProjectFormProps {
  onSuccess: () => void;
  onCancel: () => void;
}

export function NewProjectForm({ onSuccess, onCancel }: NewProjectFormProps) {
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const form = useForm<ProjectFormValues>({
    resolver: zodResolver(projectFormSchema),
    defaultValues: {
      projectName: '',
      deploymentType: 'deployment',
      contextName: '',
    },
  });

  const onSubmit = async (data: ProjectFormValues) => {
    setError('');
    setIsSubmitting(true);

    try {
      const userId = localStorage.getItem('user_id');
      
      if (!userId) {
        setError('User not authenticated. Please login again.');
        setIsSubmitting(false);
        return;
      }

      await projectAPI.createProject(
        userId,
        data.projectName,
        data.deploymentType,
        data.contextName
      );

      // Success - call the success callback
      onSuccess();
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError('Failed to create project. Please try again.');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
        {error && (
          <div className="bg-destructive/10 text-destructive text-sm p-3 rounded-md border border-destructive/20">
            {error}
          </div>
        )}

        {/* Project Name */}
        <FormField
          control={form.control}
          name="projectName"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Project Name</FormLabel>
              <FormControl>
                <Input
                  placeholder="my-awesome-project"
                  {...field}
                  disabled={isSubmitting}
                />
              </FormControl>
              <FormDescription>
                Use lowercase letters, numbers, and hyphens only
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Deployment Type */}
        <FormField
          control={form.control}
          name="deploymentType"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Deployment Type</FormLabel>
              <Select
                onValueChange={field.onChange}
                defaultValue={field.value}
                disabled={isSubmitting}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select deployment type" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value="deployment">Deployment</SelectItem>
                </SelectContent>
              </Select>
              <FormDescription>
                Select the Kubernetes resource type for your project
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Context Name */}
        <FormField
          control={form.control}
          name="contextName"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Context Name</FormLabel>
              <FormControl>
                <Input
                  placeholder="default"
                  {...field}
                  disabled={isSubmitting}
                />
              </FormControl>
              <FormDescription>
                Kubernetes context name (from your kubeconfig)
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Action Buttons */}
        <div className="flex gap-3 justify-end pt-4">
          <Button
            type="button"
            variant="outline"
            onClick={onCancel}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={isSubmitting}
            className="bg-[oklch(0.145_0_0)] text-white hover:bg-[oklch(0.2_0_0)]"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Creating...
              </>
            ) : (
              'Create Project'
            )}
          </Button>
        </div>
      </form>
    </Form>
  );
}
