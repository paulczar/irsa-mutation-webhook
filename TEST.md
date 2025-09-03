# **ROSA Pod S3 Access with S3 via IRSA (IAM Roles for Service Accounts) Tutorial**

This tutorial guides you through the process of configuring a Red Hat OpenShift Service on AWS (ROSA) pod to securely access an Amazon S3 bucket using IAM Roles for Service Accounts (IRSA). This method eliminates the need for hardcoding AWS credentials in your pod, enhancing security.

## **Prerequisites**

Before you begin, ensure you have the following:

* An active ROSA cluster.
* The oc (OpenShift CLI) and aws (AWS CLI) command-line tools installed and configured.
* IAM permissions in your AWS account to create roles, policies, and S3 buckets.

## **Step 1: Define Variables**

To simplify the process, start by defining all the necessary variables in your terminal. We will automatically retrieve your AWS Account ID and region. You can replace the other example values with your own.

```
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query "Account" --output text)
export ROSA_CLUSTER_NAME="pczarkow"
export AWS_REGION=$(rosa describe cluster -c $ROSA_CLUSTER_NAME --output json | jq -r .region.id)
export OIDC=$(oc get authentication.config.openshift.io cluster \
            -o jsonpath='{.spec.serviceAccountIssuer}' | sed  's|^https://||')
export S3_BUCKET_NAME="${ROSA_CLUSTER_NAME}s3"
export K8S_NAMESPACE="s3-app"
export K8S_SA_NAME="s3-reader"
```

## **Step 2: Create an S3 Bucket**

First, create the S3 bucket that your pod will access.

```
aws s3api create-bucket --bucket $S3_BUCKET_NAME --region $AWS_REGION --create-bucket-configuration LocationConstraint=$AWS_REGION
```

## **Step 3: Create an IAM Policy for S3 Access**

Next, create a managed IAM policy that grants the necessary permissions to access the S3 bucket.

1. Create a JSON file named s3-access-policy.json with the following content.

```
cat << EOF > /tmp/s3-access-policy.json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket"
            ],
            "Resource": "arn:aws:s3:::$S3_BUCKET_NAME"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject"
            ],
            "Resource": "arn:aws:s3:::$S3_BUCKET_NAME/*"
        }
    ]
}
EOF
```

2. Create the IAM policy using the AWS CLI.

```
aws iam create-policy --policy-name ROSA-S3-Reader-Policy --policy-document file:///tmp/s3-access-policy.json
```

3. Note the Arn from the output, as you'll need it in the next step.

```
arn:aws:iam::660250927410:policy/ROSA-S3-Reader-Policy
```

## **Step 4: Create the IAM Role and Trust Policy**

This is the core of the IRSA configuration. You'll create an IAM role with a trust policy that allows the OpenID Connect (OIDC) provider of your ROSA cluster to assume the role.

1. Get your ROSA cluster's OIDC issuer URL.

```
rosa describe cluster -c $ROSA_CLUSTER_NAME --query "oidc_endpoint_url" --output text

```

The output will be the OIDC issuer URL. Note it down. It should look something like https://oidc.eks.us-east-1.amazonaws.com/id/ABCDEFG.

2. Create a JSON file named trust-policy.json with the following content. Replace \<OIDC\_ISSUER\_URL\> with your specific OIDC endpoint URL.

```
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::$AWS_ACCOUNT_ID:oidc-provider/https://oidc.op1.openshiftapps.com/2ktrcnppbbvcs5gehecequ8jqga2jpkl"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "https://oidc.op1.openshiftapps.com/2ktrcnppbbvcs5gehecequ8jqga2jpkl:sub": "system:serviceaccount:$K8S_NAMESPACE:$K8S_SA_NAME"
        }
      }
    }
  ]
}
```

3. Create the IAM role using the trust policy.

```
aws iam create-role --role-name ROSA-S3-Reader-Role --assume-role-policy-document file://trust-policy.json

```

4. Attach the S3 access policy you created earlier to this new IAM role.

```
aws iam attach-role-policy --role-name ROSA-S3-Reader-Role --policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/ROSA-S3-Reader-Policy

```

## **Step 5: Create a Kubernetes Service Account**

Now, create a Service Account in your OpenShift cluster and annotate it with the ARN of the IAM role you just created.

1. If the namespace doesn't exist, create it:

```
oc new-project $K8S_NAMESPACE
```

2. Create a YAML file named s3-service-account.yaml with the following content.

```
cat << EOF | oc apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $K8S_SA_NAME
  namespace: $K8S_NAMESPACE
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::$AWS_ACCOUNT_ID:role/ROSA-S3-Reader-Role"
EOF
```


## **Step 6: Deploy the Pod**

Finally, create a pod that uses the service account and contains an AWS CLI image to verify access.

1. Create a YAML file named s3-test-pod.yaml.
   apiVersion: v1
   kind: Pod
   metadata:
     name: s3-test-pod
     namespace: $K8S\_NAMESPACE
   spec:
     serviceAccountName: $K8S\_SA\_NAME
     containers:
     \- name: aws-cli-container
       image: public.ecr.aws/aws-cli/aws-cli:latest
       command: \["/bin/bash"\]
       args: \["-c", "while true; do echo 'Pod is running...'; sleep 3600; do"\]
     restartPolicy: Never

2. Deploy the pod.
   oc apply \-f s3-test-pod.yaml

## **Step 7: Verify S3 Access from the Pod**

Once the pod is running, you can exec into it and use the AWS CLI to confirm it can access your S3 bucket without any credentials.

1. Check the pod's status to ensure it's Running.
   oc get pod \-n $K8S\_NAMESPACE s3-test-pod

2. Exec into the running pod.
   oc exec \-it \-n $K8S\_NAMESPACE s3-test-pod \-- /bin/bash

3. From within the pod's shell, try to list the contents of your S3 bucket.
   aws s3 ls s3://$S3\_BUCKET\_NAME

If the command succeeds and lists the contents of your bucket, you have successfully configured IRSA\! This demonstrates that the pod has assumed the IAM role and is using its temporary credentials to access the bucket.

To clean up, you can delete the pod and other resources:

oc delete pod s3-test-pod \-n $K8S\_NAMESPACE
oc delete sa $K8S\_SA\_NAME \-n $K8S\_NAMESPACE
aws iam detach-role-policy \--role-name ROSA-S3-Reader-Role \--policy-arn arn:aws:iam::$AWS\_ACCOUNT\_ID:policy/ROSA-S3-Reader-Policy
aws iam delete-role \--role-name ROSA-S3-Reader-Role
aws iam delete-policy \--policy-arn arn:aws:iam::$AWS\_ACCOUNT\_ID:policy/ROSA-S3-Reader-Policy
aws s3 rb s3://$S3\_BUCKET\_NAME \--force

This tutorial provides a clear path from IAM role creation to a functional pod with secure S3 access.