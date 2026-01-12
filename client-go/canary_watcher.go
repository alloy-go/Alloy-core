package services

import (
    "log"
    "gopkg.in/yaml.v3"
)

// Modify deployment for canary (name, replicas, image, track label)
func modifyDeploymentForCanary(yamlContent []byte, name, image string, replicas int, track string) []byte {
    var dep map[string]interface{}
    if err := yaml.Unmarshal(yamlContent, &dep); err != nil {
        log.Printf("❌ YAML unmarshal failed: %v", err)
        return yamlContent
    }
    
    // Update metadata.name
    if metadata, ok := dep["metadata"].(map[string]interface{}); ok {
        metadata["name"] = name
    }
    
    // Get spec
    spec, ok := dep["spec"].(map[string]interface{})
    if !ok {
        log.Printf("❌ No spec found in deployment YAML")
        return yamlContent
    }
    
    // Update spec.replicas
    spec["replicas"] = replicas
    
    // Get original app label from selector
    appLabel := "app"  // default
    if selector, ok := spec["selector"].(map[string]interface{}); ok {
        if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
            // Get the app label value (usually "student-app" or similar)
            if appVal, ok := matchLabels["app"].(string); ok {
                appLabel = appVal
            }
            // Update selector to include track
            matchLabels["track"] = track
        }
    }
    
    // Update pod template labels to match selector
    if template, ok := spec["template"].(map[string]interface{}); ok {
        if metadataT, ok := template["metadata"].(map[string]interface{}); ok {
            // Ensure labels exist
            if labels, ok := metadataT["labels"].(map[string]interface{}); ok {
                labels["app"] = appLabel
                labels["track"] = track
            } else {
                // Create labels if they don't exist
                metadataT["labels"] = map[string]interface{}{
                    "app":   appLabel,
                    "track": track,
                }
            }
        }
        
        // Update container image
        if podSpec, ok := template["spec"].(map[string]interface{}); ok {
            if containers, ok := podSpec["containers"].([]interface{}); ok && len(containers) > 0 {
                if cont, ok := containers[0].(map[string]interface{}); ok {
                    cont["image"] = image
                }
            }
        }
    }
    
    result, err := yaml.Marshal(dep)
    if err != nil {
        log.Printf("❌ YAML marshal failed: %v", err)
        return yamlContent
    }
    
    log.Printf("📝 Generated YAML for %s (%d replicas, track=%s)", name, replicas, track)
    return result
}

// NEW: Update existing stable deployment's image WITHOUT changing selector
// This avoids the "selector is immutable" error
func updateStableDeploymentImage(yamlContent []byte, newImage string, replicas int) []byte {
    var dep map[string]interface{}
    if err := yaml.Unmarshal(yamlContent, &dep); err != nil {
        log.Printf("❌ YAML unmarshal failed: %v", err)
        return yamlContent
    }
    
    // Update spec.replicas
    if spec, ok := dep["spec"].(map[string]interface{}); ok {
        spec["replicas"] = replicas
        
        // Update container image ONLY (don't touch selector or labels)
        if template, ok := spec["template"].(map[string]interface{}); ok {
            if podSpec, ok := template["spec"].(map[string]interface{}); ok {
                if containers, ok := podSpec["containers"].([]interface{}); ok && len(containers) > 0 {
                    if cont, ok := containers[0].(map[string]interface{}); ok {
                        cont["image"] = newImage
                    }
                }
            }
        }
    }
    
    result, err := yaml.Marshal(dep)
    if err != nil {
        log.Printf("❌ YAML marshal failed: %v", err)
        return yamlContent
    }
    
    log.Printf("📝 Updated stable deployment image to %s (%d replicas)", newImage, replicas)
    return result
}

// Modify service to route to both canary and stable pods
// This is CRITICAL for canary deployments - removes "track" label from service selector
// so it routes traffic to BOTH stable and canary pods simultaneously
func modifyServiceForCanary(yamlContent []byte) []byte {
    var svc map[string]interface{}
    if err := yaml.Unmarshal(yamlContent, &svc); err != nil {
        log.Printf("❌ Service YAML unmarshal failed: %v", err)
        return yamlContent
    }
    
    // Update service selector to only use "app" label (not track)
    // This allows it to route to BOTH stable and canary pods
    if spec, ok := svc["spec"].(map[string]interface{}); ok {
        if selector, ok := spec["selector"].(map[string]interface{}); ok {
            // Remove track label from selector if it exists
            delete(selector, "track")
            log.Printf("📝 Service selector updated to route to all tracks (canary + stable)")
        }
    }
    
    result, err := yaml.Marshal(svc)
    if err != nil {
        log.Printf("❌ Service YAML marshal failed: %v", err)
        return yamlContent
    }
    
    return result
}